# -*- mode: Python -*-

kubectl_cmd = "kubectl"
flux_cmd = "flux"

# verify kubectl command exists
if str(local("command -v " + kubectl_cmd + " || true", quiet = True)) == "":
    fail("Required command '" + kubectl_cmd + "' not found in PATH")

# set defaults
settings = {
    "flux": {
        "enabled": False,
        "bootstrap": True,
        "repository": os.getenv("FLUX_REPOSITORY", "podinfo-flux-example"),
        "owner": os.getenv("FLUX_OWNER", ""),
        "path": os.getenv("FLUX_PATH", "."),
    },
    "install_unpacker": {
        "enabled": False,
        "path": "",
    },
    "debug": {
        "enabled": False,
    },
    "create_secrets": {
        "enable": True,
        "token": os.getenv("GITHUB_TOKEN", ""),
        "email": os.getenv("GITHUB_EMAIL", ""),
        "user": os.getenv("GITHUB_USER", ""),
    },
    "forward_registry": False,
    "verification_keys": {},
}

# global settings
tilt_file = "./tilt-settings.yaml" if os.path.exists("./tilt-settings.yaml") else "./tilt-settings.json"
settings.update(read_yaml(
    tilt_file,
    default = {},
))
load('ext://secret', 'secret_yaml_registry', 'secret_from_dict', 'secret_create_generic')


def bootstrap_or_install_flux():
    opts = settings.get("flux")
    if not opts.get("enabled"):
        return

    if str(local("command -v " + flux_cmd + " || true", quiet = True)) == "":
        fail("Required command '" + flux_cmd + "' not found in PATH")

    # flux bootstrap github --owner=${FLUX_OWNER} --repository=${FLUX_REPOSITORY} --path ${FLUX_PATH}
    if opts.get("bootstrap"):
        local("%s bootstrap github --owner %s --repository %s --path %s" % (flux_cmd, opts.get('owner'), opts.get('repository'), opts.get('path')))
    else:
        local(flux_cmd + " install")


def install_unpacker():
    opts = settings.get("install_unpacker")
    if not opts.get("enabled"):
        return


def create_secrets():
    opts = settings.get("create_secrets")
    if not opts.get("enable"):
        return

    k8s_yaml(secret_from_dict("creds", "ocm-system", inputs = {
        'username' : opts.get('user'),
        'password' : opts.get('token'),
    }), allow_duplicates = True)


def create_verification_keys():
    keys = settings.get("verification_keys")
    if not keys:
        return

    for key, value in keys.items():
        secret_create_generic(key, 'ocm-system', from_file=value)

# set up the development environment

# check if flux is needed
bootstrap_or_install_flux()

# check if installing unpacker is needed
install_unpacker()

print('applying generated secrets')
k8s_yaml('./hack/certs/registry_certs_secret.yaml')

# Use kustomize to build the install yaml files
install = kustomize('config/default')

# Update the root security group. Tilt requires root access to update the
# running process.
objects = decode_yaml_stream(install)
for o in objects:
    if o.get('kind') == 'Deployment' and o.get('metadata').get('name') == 'ocm-controller':
        o['spec']['template']['spec']['securityContext']['runAsNonRoot'] = False
        if settings.get('debug').get('enabled'):
            o['spec']['template']['spec']['containers'][0]['ports'] = [{'containerPort': 30000}]
        print('updating ocm-controller deployment to add generated certificates')
        o['spec']['template']['spec']['volumes'] = [{'name': 'root-certificate', 'secret': {'secretName': 'registry-certs', 'items': [{'key': 'caFile', 'path': 'ca.pem'}]}}]
        o['spec']['template']['spec']['containers'][0]['volumeMounts'] = [{'mountPath': '/certs', 'name': 'root-certificate'}]

    if o.get('kind') == 'Deployment' and o.get('metadata').get('name') == 'registry':
        print('updating registry deployment to add generated certificates')
        o['spec']['template']['spec']['containers'][0]['env'] += [{'name': 'REGISTRY_HTTP_TLS_CERTIFICATE', 'value': '/certs/cert.pem'}, {'name': 'REGISTRY_HTTP_TLS_KEY', 'value': '/certs/key.pem'}, {'name': 'REGISTRY_HTTP_TLS_CLIENTCAS_0', 'value': '/certs/ca.pem'}]
        o['spec']['template']['spec']['containers'][0]['volumeMounts'] = [{'mountPath': '/certs', 'name': 'registry-certs'}]
        o['spec']['template']['spec']['volumes'] = [{'name': 'registry-certs', 'secret': {'secretName': 'registry-certs', 'items': [{'key': 'certFile', 'path': 'cert.pem'}, {'key': 'keyFile', 'path': 'key.pem'}, {'key': 'caFile', 'path': 'ca.pem'}]}}]

updated_install = encode_yaml_stream(objects)

# Apply the updated yaml to the cluster.
k8s_yaml(updated_install, allow_duplicates = True)

# Create Secrets
create_secrets()
create_verification_keys()

load('ext://restart_process', 'docker_build_with_restart')

# enable hot reloading by doing the following:
# - locally build the whole project
# - create a docker imagine using tilt's hot-swap wrapper
# - push that container to the local tilt registry
# Once done, rebuilding now should be a lot faster since only the relevant
# binary is rebuilt and the hot swat wrapper takes care of the rest.
gcflags = ''
if settings.get('debug').get('enabled'):
    gcflags = '-N -l'

local_resource(
    'ocm-controller-binary',
    "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags '{gcflags}' -o bin/manager ./".format(gcflags=gcflags),
    deps = [
        "main.go",
        "go.mod",
        "go.sum",
        "api",
        "controllers",
        "pkg",
        "hack/entrypoint.sh",
    ],
)


# Build the docker image for our controller. We use a specific Dockerfile
# since tilt can't run on a scratch container.
# `only` here is important, otherwise, the container will get updated
# on _any_ file change. We only want to monitor the binary.
# If debugging is enabled, we switch to a different docker file using
# the delve port.
entrypoint = ['/entrypoint.sh', '/manager']
dockerfile = 'tilt.dockerfile'
if settings.get('debug').get('enabled'):
    k8s_resource('ocm-controller', port_forwards=[
        port_forward(30000, 30000, 'debugger'),
    ])
    entrypoint = ['/dlv', '--listen=:30000', '--api-version=2', '--continue=true', '--accept-multiclient=true', '--headless=true', 'exec', '/manager', '--']
    dockerfile = 'tilt.debug.dockerfile'


docker_build_with_restart(
    'ghcr.io/open-component-model/ocm-controller',
    '.',
    dockerfile = dockerfile,
    entrypoint = entrypoint,
    only=[
      './bin',
      './hack/entrypoint.sh',
    ],
    live_update = [
        sync('./hack/entrypoint.sh', '/entrypoint.sh'),
        sync('./bin/manager', '/manager'),
    ],
)


if settings.get('forward_registry'):
    k8s_resource('ocm-controller', extra_pod_selectors = [{'app': 'registry'}], port_forwards='5000:5000')
