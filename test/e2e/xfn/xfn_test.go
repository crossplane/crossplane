//go:build integration
// +build integration

/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package xfn

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/crossplane/crossplane/apis/apiextensions/fn/proto/v1alpha1"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/util/wait"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strings"
	"testing"
	"time"
)

var xfnImageRef *string = flag.String("xfn-image-ref", "", "xfn runner image ref")

func TestCustomCA(t *testing.T) {
	cases := map[string]struct {
		body func(xfnImageRef, certDir string) error
	}{
		"retrieve xfn image from private registry by setting CA cert via CLI arg": {
			body: func(fnImageRef, certDir string) error {
				// start xfn runner container
				cmd := exec.Command("docker",
					"run", "--rm", "-i",
					// making that registry appears as foo.bar host inside xfn container
					"--link", "registry:foo.bar",
					"-v", fmt.Sprintf("%s:/certs:z", certDir),
					*xfnImageRef,
					"spark", "--dry-run=true", "--ca-bundle-path", "/certs/domain.crt")
				return dryRun(fnImageRef, cmd)
			},
		},
		"retrieve xfn image from private registry by setting CA cert via env variable": {
			body: func(fnImageRef, certDir string) error {
				cmd := exec.Command("docker",
					"run", "--rm", "-i",
					// making that registry appears as foo.bar host inside xfn container
					"--link", "registry:foo.bar",
					"-v", fmt.Sprintf("%s:/certs:z", certDir),
					"-e", "CA_BUNDLE_PATH=/certs/domain.crt",
					*xfnImageRef,
					"spark", "--dry-run=true")
				return dryRun(fnImageRef, cmd)
			},
		},
		"xfn image pull fails if CA cert not set": {
			body: func(fnImageRef, certDir string) error {
				cmd := exec.Command("docker",
					"run", "--rm", "-i",
					// making that registry appears as foo.bar host inside xfn container
					"--link", "registry:foo.bar",
					"-v", fmt.Sprintf("%s:/certs:z", certDir),
					*xfnImageRef,
					"spark", "--dry-run=true")
				err := dryRun(fnImageRef, cmd)
				if err == nil {
					return fmt.Errorf("Image pull should fail, but it did not")
				}
				return nil
			},
		},
	}
	reg := &ociRegistry{Port: 5000, DataDir: t.TempDir()}
	certDir := t.TempDir()
	t.Logf("registry datadir %s", reg.DataDir)
	t.Logf("certdir %s", certDir)

	// start local registry
	err := reg.Start()

	defer reg.Stop()

	if err != nil {
		t.Fatal(err)
	}

	// copy an image from dockerhub into the localregistry
	err = copyImage("docker.io/hello-world:latest", "localhost:5000/h-w:latest")
	if err != nil {
		t.Fatal(err)
	}

	// stop registry, but keep the images for the second registry start
	err = reg.Stop()

	if err != nil {
		t.Fatal(err)
	}
	// now, we start the registry again with TLS enable, using custom generated certificate
	reg.CertDir = certDir
	err = reg.Start()
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.body("foo.bar:5000/h-w:latest", reg.CertDir)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type ociRegistry struct {
	Port    int
	DataDir string
	CertDir string
}

func (r *ociRegistry) Start() error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	args := []string{
		"run",
		"-d",
		"-u", u.Uid,
		"-p", fmt.Sprintf("%d:%d", r.Port, r.Port),
		"-v", fmt.Sprintf("%s:/var/lib/registry", r.DataDir),
	}
	if r.CertDir != "" {
		args = append(args,
			"-v", fmt.Sprintf("%s:/certs:z", r.CertDir),
			"-e", "REGISTRY_HTTP_TLS_CERTIFICATE=/certs/domain.crt",
			"-e", "REGISTRY_HTTP_TLS_KEY=/certs/domain.key")
		if err = createCerts(r.CertDir); err != nil {
			return err
		}
	}
	args = append(args, "--name", "registry", "registry:2")
	cmd := exec.Command("docker", args...)
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return waitPortRead(r.Port)
}

func (r *ociRegistry) Stop() error {
	_, err := exec.Command("docker", "rm", "-f", "registry").Output()
	return err
}

func dryRun(fnImageRef string, cmd *exec.Cmd) error {
	fnStdin := "fooInput"
	req := &v1alpha1.RunFunctionRequest{
		Image: fnImageRef,
		Input: []byte(fnStdin),
	}

	out, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	cmd.Stdin = bytes.NewReader(out)
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	out, err = cmd.Output()
	if err != nil {
		fmt.Println(stderr.String())
		return err
	}
	fmt.Println(stderr.String())
	rsp := &v1alpha1.RunFunctionResponse{}
	err = proto.Unmarshal(out, rsp)
	if err != nil {
		return err
	}
	if string(rsp.Output) != fnStdin {
		return fmt.Errorf("Expected %s, got %s", fnStdin, rsp.Output)
	}
	return nil
}

func copyImage(srcRef, dstRef string) error {
	_, err := exec.Command("docker", "pull", srcRef).Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("docker", "tag", srcRef, dstRef).Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("docker", "push", dstRef).Output()
	return err
}

func waitPortRead(port int) error {
	return wait.PollImmediate(2*time.Second, 20*time.Second, func() (done bool, err error) {
		addr := net.JoinHostPort("localhost", fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			return false, nil
		}
		if conn != nil {
			_ = conn.Close()
			return true, nil
		}
		return false, nil
	})
}

func createCerts(certDir string) error {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
			CommonName:    "foo.bar",
		},
		DNSNames:              []string{"foo.bar"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	f, err := os.Create(path.Join(certDir, "domain.crt"))
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(caPEM.String())

	keyPEM := new(bytes.Buffer)
	pem.Encode(keyPEM, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	f, err = os.Create(path.Join(certDir, "domain.key"))
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(keyPEM.String())

	return nil
}
