# Tilt >= v0.17.8 is required to handle escaping of colons in selector names and proper
# teardown of resources
load('ext://min_tilt_version', 'min_tilt_version')
min_tilt_version('0.17.8')

# We require at minimum CRD support, so need at least Kubernetes v1.16
load('ext://min_k8s_version', 'min_k8s_version')
min_k8s_version('1.16')

# Load the extension for live updating
load('ext://restart_process', 'docker_build_with_restart')

# Load the provider helpers
load('cluster/local/provider.Tiltfile', 'build_provider', 'deploy_provider')

# For consistency we're going to always create 'crossplane-system' namespace and use it
load('ext://namespace', 'namespace_create')
namespace = "crossplane-system"
namespace_create(namespace)

####################################################################################
# Crossplane Core
####################################################################################
# Main function to make sure inital build steps runs in sequence
def main():
    generate_crds()
    build_crossplane()
    deploy_crossplane()

# Build crossplane binary and docker image. Binary is built for linux-amd64 (to be
# only used inside a container.) The binary is stored in crossplane cloned folder
# at _output/tilt/
def build_crossplane():
    # Build crossplane
    local_resource(
        'build-crossplane',
        'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o _output/tilt/crossplane ./cmd/crossplane',
        deps = [
            'go.mod',
            'go.sum',
            'cmd/crossplane',
            'apis',
            'internal'
        ],
        ignore = [
            'apis/**/zz_generated.*',
            'internal/client/clientset'
        ]
    )

    dockerfile_contents = '\n'.join([
        'FROM alpine:3.7',
        'RUN apk --no-cache add ca-certificates bash',
        'ADD _output/tilt/crossplane /usr/local/bin/crossplane',
        'EXPOSE 8080',
        'ENTRYPOINT ["crossplane"]'
    ])

    # Build crossplane docker image
    docker_build_with_restart(
        'crossplane/crossplane',
        '.',
        dockerfile_contents = dockerfile_contents,
        only = [
            '_output/tilt/crossplane',
        ],
        live_update = [
            sync('_output/tilt/crossplane', '/usr/local/bin/crossplane')
        ],
        entrypoint = [
            'crossplane'
        ]
    )

# Regenerate CRDs if content of 'api/' folder changes. Note that all the 'zz_generated.*'
# files are excluded from retriggering the build again, otherwise we would end up in an
# infinite loop of building. But 'zz_generated.*' files will be included in the final
# image and will get live reloaded into running pod.
def generate_crds():
    # Generate crossplane CRDs out of API definition
    _packages = ' '.join([
        'github.com/crossplane/crossplane/cmd/...',
        'github.com/crossplane/crossplane/internal/...',
        'github.com/crossplane/crossplane/apis/...'
    ])
    local_resource(
        'gen-crds',
        'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go generate -tags "generate" ' + _packages,
        deps = [
            'apis'
        ],
        ignore = [
            'apis/**/zz_generated.*'
        ]
    )

    # Find all the CRDs manifest files
    #
    # NOTE(khos2ow): we don't need to manually apply generated CRDs
    # here. They will get re-applied through helm (see below).
    crds_folder = 'cluster/charts/crossplane/crds'
    files = listdir(crds_folder)

    # Extract name of the CRDs to show on Tilt UI
    crds_name = []
    for f in files:
        crds_name.append(read_yaml(f)['metadata']['name'] + ':customresourcedefinition')

    crds_name.append('lock:lock')

    # Group CRDs together for easy access on Tilt UI
    k8s_resource(
        new_name = 'crds',
        objects = crds_name,
        resource_deps = [
            'crossplane'
        ]
    )

def deploy_crossplane():
    # Deploy the built chart with just built crossplane image
    k8s_yaml(helm(
        'cluster/charts/crossplane',
        name = 'crossplane',
        namespace = namespace,
        set = [
            # NOTE(khos2ow): modifying security context to be able to
            # live reload the changes into pods.
            #
            # DO NOT SET THESE VALUES IN PRODUCTION!!
            #
            'securityContextCrossplane.runAsUser=0',
            'securityContextCrossplane.runAsGroup=0',
            'securityContextCrossplane.readOnlyRootFilesystem=false',
            'securityContextRBACManager.readOnlyRootFilesystem=false',

            # NOTE(khos2ow): explicitly disabling leader election for
            # development setup, because it will take a bit of time to
            # completely acquire a lock and that causes a race condition
            # on deploying proviers down below (if any).
            'leaderElection=false',
            'rbacManager.leaderElection=false'
        ]
    ))

    # Group service accounts and cluster roles together for easy access on Tilt UI
    k8s_resource(
        new_name = 'roles-accounts',
        objects = [
            'crossplane:serviceaccount',
            'rbac-manager:serviceaccount',
            'crossplane:clusterrole',
            'crossplane:clusterrolebinding',
            'crossplane\\:system\\:aggregate-to-crossplane:clusterrole',
            'crossplane-rbac-manager:clusterrole',
            'crossplane-rbac-manager:clusterrolebinding',
            'crossplane-admin:clusterrolebinding',
            'crossplane-admin:clusterrole',
            'crossplane-edit:clusterrole',
            'crossplane-view:clusterrole',
            'crossplane-browse:clusterrole',
            'crossplane\\:aggregate-to-admin:clusterrole',
            'crossplane\\:aggregate-to-edit:clusterrole',
            'crossplane\\:aggregate-to-view:clusterrole',
            'crossplane\\:aggregate-to-ns-admin:clusterrole',
            'crossplane\\:aggregate-to-ns-edit:clusterrole',
            'crossplane\\:aggregate-to-ns-view:clusterrole'
        ]
    )

main()

####################################################################################
# Crossplane Providers
#
# Users can use tilt-providers.json to build and deploy additional providers
# into the cluster. There are two ways of doing so:
#
# 1. deploy existing official providers (from DockerHub)
# 2. build and deploy local clone of a provider from filesystem
#
# Example content of tilt-providers.json:
# 
# {
#     "provider-foo": {
#         "enabled": true,
#         "context": "/path/to/provider-cloned-folder"
#     },
#     "provider-bar": {
#         "enabled": false,
#         "package_owner": "crossplane",
#         "package_ref": "alpha"
#     }
# }
#
# Example content of tilt-settings.json:
#
# {
#     "args": [],                        // args to pass to pod
#     "debug": false,                    // enable debug mode
#     "namespace": "crossplane-system"   // namespace to deploy provider into
# }
#
# Note that `context` can also be relative to crossplane local cloned folder.
####################################################################################
def build_deploy_providers():
    settings = {
        'args': [],
        'debug': False,
        'namespace': 'crossplane-system'
    }
    settings.update(read_json(
        'tilt-settings.json',
        default = {}
    ))

    # Make sure these value are set as follow (they are needed like this)
    settings['core_path'] = os.getcwd()
    settings['resource_deps'] = [
        'crossplane',
        'crossplane-rbac-manager',
        'crds'
    ]

    providers = {}
    providers.update(read_json(
        'tilt-providers.json',
        default = {}
    ))

    for name in providers:
        provider = providers.get(name)
        provider['short_name'] = name.replace('provider-', '')

        if provider.get('enabled') != True :
            continue

        if provider.get('context') != None :
            config_file = os.path.join(provider.get('context'), 'tilt-provider.json')
            if os.path.exists(config_file) == False:
                continue

            provider_config = {}
            provider_config.update(read_json(
                config_file,
                default = {}
            ))

            for key in provider_config:
                provider[key] = provider_config[key]
            
            settings['local_image'] = True
            build_provider(name, provider, settings)

        deploy_provider(name, provider, settings)

build_deploy_providers()

####################################################################################
# Custom Tiltfiles
#
# Users may define their own Tilt customizations in tilt.d. This directory is
# excluded from git and these files will not be checked in to version control.
####################################################################################
def include_custom_files():
    for f in listdir("tilt.d"):
        include(f)

include_custom_files()
