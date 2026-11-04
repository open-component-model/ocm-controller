# Image localization

A `ConfigData` localization rule writes parts of an OCM resource's current image
access reference into YAML or JSON files. Each property contains a destination
path, such as `image.repository`, rather than the literal value to write.

| Property | Value written to the destination path |
| --- | --- |
| `image` | Fully qualified image reference. A digest-pinned reference uses `repository@digest`. |
| `registry` | Registry hostname, including its port when present. |
| `repository` | Repository path without the registry. |
| `fullyQualifiedRepository` | Registry and repository path without tag or digest. |
| `tag` | Tag for a tagged reference; `tag@digest` when both are present. Digest-only references produce their digest here. |
| `digest` | Algorithm-prefixed digest, such as `sha256:…`, without `@`. The reference must contain a digest. |

## Charts with separate repository and tag values

A chart may construct its image with:

```gotemplate
image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
```

Use this localization configuration:

```yaml
apiVersion: config.ocm.software/v1alpha1
kind: ConfigData
metadata:
  name: image-localization
localization:
- file: values.yaml
  fullyQualifiedRepository: image.repository
  tag: image.tag
  resource:
    name: application-image
```

For a resource with the image reference
`registry.example/team/image:v1@sha256:…`, the resulting values are:

```yaml
image:
  repository: registry.example/team/image
  tag: v1@sha256:…
```

The chart renders `registry.example/team/image:v1@sha256:…`, retaining the digest
pin. The localized `tag` is a suffix suitable for this chart expression, rather
than a standalone OCI tag when it includes `@digest`.

Digest examples in this document are abbreviated. Actual references must contain
a complete digest.

## Charts with a dedicated digest value

For a chart that constructs `repository@digest`, use:

```yaml
localization:
- file: values.yaml
  fullyQualifiedRepository: image.repository
  digest: image.digest
  resource:
    name: application-image
```

The resulting values are:

```yaml
image:
  repository: registry.example/team/image
  digest: sha256:…
```

The chart can render:

```gotemplate
image: "{{ .Values.image.repository }}@{{ .Values.image.digest }}"
```

This works with both digest-only and tag-and-digest source references. Requesting
`digest` for a tag-only reference fails reconciliation with `no digest is present`.
Localization does not contact the image registry to calculate a missing digest.
Do not append another digest to a localized `tag` that already contains `@digest`.

When a chart accepts a complete image reference, `image` localization can write
that value directly instead of splitting it into repository and tag or digest.

## Reference forms

| Source reference | `tag` output | `digest` output |
| --- | --- | --- |
| `repository:v1` | `v1` | Error: no digest is present. |
| `repository:v1@sha256:…` | `v1@sha256:…` | `sha256:…` |
| `repository@sha256:…` | `sha256:…` | `sha256:…` |
| `repository` | `latest` | Error: no digest is present. |

A digest-only `tag` value does not form a valid image when joined to a repository
with `:`. Use `image` or a dedicated `digest` value for digest-only references.
A chart that requires a tag needs a source reference with a tag.

The `digest` property is optional and extracts the digest from either a digest-only
or a tag-and-digest reference. `ConfigData` is embedded configuration rather than
a Kubernetes custom resource.
