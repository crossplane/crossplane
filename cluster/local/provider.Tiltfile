####################################################################################
# Crossplane Providers helper functions
#
# These are helper functions to build, deploy, and live-reload providers.
# Note that these functions are shared with both:
#
# 1) core crossplane repo
# 2) any standalone provider
####################################################################################

# Register CRDs which expose `image` for Tilt to be able to find them and
# build the corresponding docker image for it.
# https://docs.tilt.dev/custom_resource.html#does-it-contain-images
k8s_kind('ControllerConfig', image_json_path = '{.spec.image}')

# Build a crossplane provider
#
# 1. build and apply CRDs
# 2. build and deploy controller manager
def build_provider(name, provider, settings):
    _generate_crds(provider, settings)
    _build_controller(name, provider, settings)

# Generate and apply crossplane provider's CRDs
def _generate_crds(provider, settings):
    context = provider.get('context')

    # Prefix each dependency with context so Tilt can watch the correct paths
    # for changes. At the same time keep track of 'zz_generated.*' files to
    # ignore (this is needed to prevent being stuck in an infinite loop.)
    crd_deps = []
    ignore_deps = []
    for d in provider.get('crd_deps', []):
        crd_deps.append(context + '/' + d)
        ignore_deps.append(context + '/' + d + '/**/zz_generated.*')
        
    # Generate crossplane provider CRDs out of API definition
    local_resource(
        'gen-crds-' + provider['short_name'],
        'cd ' + context + ' && make generate',
        deps = crd_deps,
        ignore = ignore_deps
    )

    # Find all the CRDs manifests files
    crds_folder = context + '/' + provider.get('crds_folder', 'package/crds')
    files = listdir(crds_folder)

    # Extract name of the CRDs to show on Tilt UI
    crds_name = []
    for f in files:
        crds_name.append(read_yaml(f)['metadata']['name'] + ':customresourcedefinition')

    # Apply CRD manifests
    k8s_yaml(files)
    k8s_resource(
        new_name = 'crds-' + provider['short_name'],
        objects = crds_name,
        resource_deps = settings.get('resource_deps')
    )

# Build crossplane provider binary and docker image. Binary is built for
# linux-amd64 (to be only used inside a container.) The binary is stored
# in crossplane cloned folder at _output/tilt/
def _build_controller(name, provider, settings):
    context = provider.get('context')
    go_main = provider.get('go_main')
    core_path = settings.get('core_path')

    # Prefix each live reload dependency with context so Tilt
    # can watch the correct paths for changes
    cmd_deps = []
    ignore_deps = []
    for d in provider.get('cmd_deps', []):
        cmd_deps.append(context + '/' + d)
        ignore_deps.append(context + '/' + d + '/**/zz_generated.*')

    # Build crossplane provider
    build_output = '_output/tilt/' + name
    local_resource(
        'build-' + provider['short_name'],
        'cd ' + context + ' && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ' + core_path + '/' + build_output + ' ' + go_main,
        deps = cmd_deps,
        ignore = ignore_deps
    )

    # Build crossplane provider docker image
    dockerfile_contents = '\n'.join([
        'FROM alpine:3.7',
        'RUN apk --no-cache add ca-certificates bash',
        'ADD ' + build_output + ' /usr/local/bin/crossplane-provider',
        # TODO(khos2ow): change this URL right before
        # merging the PR in crossplane/crossplane!
        #
        # https://github.com/crossplane/crossplane/pull/1925
        'ADD https://raw.githubusercontent.com/khos2ow/crossplane/tilt/cluster/local/tilt_start.sh /usr/local/bin',
        'ADD https://raw.githubusercontent.com/khos2ow/crossplane/tilt/cluster/local/tilt_restart.sh /usr/local/bin',
        'RUN chmod +x /usr/local/bin/tilt_start.sh /usr/local/bin/tilt_restart.sh',
        'EXPOSE 8080',
        'ENTRYPOINT ["tilt_start.sh", "crossplane-provider"]'
    ])

    package_owner = provider.get('package_owner', 'crossplane')

    docker_build(
        provider.get('image_name', package_owner + '/' + name + '-controller'),
        '.',
        dockerfile_contents = dockerfile_contents,
        only = [
            build_output,
        ],
        live_update = [
            sync(build_output, '/usr/local/bin/crossplane-provider'),
            run('tilt_restart.sh'),
        ]
    )

# Deploy crossplane provider into the cluster. If local_image is set to 'True' an
# additional 'ControllerConfig' also gets deployed with override 'spec.image' for
# Tilt to be able to inject the locally built provider image into the Deployment.
def deploy_provider(name, provider, settings):
    package_owner = provider.get('package_owner', 'crossplane')
    package_ref = provider.get('package_ref', 'master')
    package_name = provider.get('package_name', package_owner + '/' + name + ':' + package_ref)

    package_revision = local(
        'docker pull ' + package_name + ' | grep "Digest:" | cut -d: -f3 | head -c 12',
        quiet = True
    )
    provider['package_revision'] = str(package_revision)

    spec = {
        'package': package_name
    }

    if settings.get('local_image', False) == True:
        _deploy_controller_config(name, provider, settings)

        spec['controllerConfigRef'] = {
            'name': provider['short_name']
        }

    k8s_yaml(encode_yaml({
        'apiVersion': 'pkg.crossplane.io/v1',
        'kind': 'Provider',
        'metadata': {
            'name': name,
            'namespace': settings.get('namespace'),
        },
        'spec': spec
    }))
    k8s_resource(
        new_name = name,
        objects = [
            name + ':provider:' + settings.get('namespace'),
        ],
        resource_deps = settings.get('resource_deps')
    )

# Deploy a ControllerConfig with overrided spec.image
def _deploy_controller_config(name, provider, settings):
    package_owner = provider.get('package_owner', 'crossplane')
    image_name = provider.get('image_name', package_owner + '/' + name + '-controller:master')

    args = settings.get('args', [])
    if settings.get('debug') == True:
        args.append('--debug')

    k8s_yaml(encode_yaml({
        'apiVersion': 'pkg.crossplane.io/v1alpha1',
        'kind': 'ControllerConfig',
        'metadata': {
            'name': provider['short_name'],
            'namespace': settings.get('namespace'),
        },
        'spec': {
            'image': image_name,
            'args': args,
            'podSecurityContext': {
                'runAsUser': 0,
                'runAsGroup': 0
            },
            'securityContext': {
                'runAsUser': 0,
                'runAsGroup': 0,
                'readOnlyRootFilesystem': False
            }
        }
    }))
    k8s_resource(
        workload = provider['short_name'],
        resource_deps = settings.get('resource_deps'),
        extra_pod_selectors = [{
            'pkg.crossplane.io/revision': name + '-' + provider.get('package_revision')
        }]
    )
