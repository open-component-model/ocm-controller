<h1>OCM Controller API reference v1alpha1</h1>
<p>Packages:</p>
<ul class="simple">
<li>
<a href="#delivery.ocm.software%2fv1alpha1">delivery.ocm.software/v1alpha1</a>
</li>
</ul>
<h2 id="delivery.ocm.software/v1alpha1">delivery.ocm.software/v1alpha1</h2>
<p>Package v1alpha1 contains API Schema definitions for the delivery v1alpha1 API group</p>
Resource Types:
<ul class="simple"></ul>
<h3 id="delivery.ocm.software/v1alpha1.ComponentDescriptor">ComponentDescriptor
</h3>
<p>ComponentDescriptor is the Schema for the componentdescriptors API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ComponentDescriptorSpec">
ComponentDescriptorSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>ComponentVersionSpec</code><br>
<em>
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1.ComponentVersionSpec
</em>
</td>
<td>
<p>
(Members of <code>ComponentVersionSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ComponentDescriptorStatus">
ComponentDescriptorStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ComponentDescriptorSpec">ComponentDescriptorSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentDescriptor">ComponentDescriptor</a>)
</p>
<p>ComponentDescriptorSpec adds a version to the top level component descriptor definition.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ComponentVersionSpec</code><br>
<em>
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1.ComponentVersionSpec
</em>
</td>
<td>
<p>
(Members of <code>ComponentVersionSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ComponentDescriptorStatus">ComponentDescriptorStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentDescriptor">ComponentDescriptor</a>)
</p>
<p>ComponentDescriptorStatus defines the observed state of ComponentDescriptor</p>
<h3 id="delivery.ocm.software/v1alpha1.ComponentVersion">ComponentVersion
</h3>
<p>ComponentVersion is the Schema for the ComponentVersions API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionSpec">
ComponentVersionSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>component</code><br>
<em>
string
</em>
</td>
<td>
<p>Every Component Version has a name.
Name and version are the identifier for a Component Version and therefor for the artifact set described by it.
A component name SHOULD reference a location where the component’s resources (typically source code, and/or documentation) are hosted.
It MUST be a DNS compliant name with lowercase characters and MUST contain a name after the domain.
Examples:
- github.com/pathToYourRepo</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Version">
Version
</a>
</em>
</td>
<td>
<p>Component versions refer to specific snapshots of a component. A common scenario being the release of a component.</p>
</td>
</tr>
<tr>
<td>
<code>repository</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Repository">
Repository
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>verify</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Signature">
[]Signature
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>references</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ReferencesConfig">
ReferencesConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this component version object.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountName can be used to configure access to both destination and source repositories.
If service account is defined, it&rsquo;s usually redundant to define access to either source or destination, but
it is still allowed to do so.
<a href="https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account">https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account</a></p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionStatus">
ComponentVersionStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ComponentVersionSpec">ComponentVersionSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersion">ComponentVersion</a>)
</p>
<p>ComponentVersionSpec defines the desired state of ComponentVersion</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>component</code><br>
<em>
string
</em>
</td>
<td>
<p>Every Component Version has a name.
Name and version are the identifier for a Component Version and therefor for the artifact set described by it.
A component name SHOULD reference a location where the component’s resources (typically source code, and/or documentation) are hosted.
It MUST be a DNS compliant name with lowercase characters and MUST contain a name after the domain.
Examples:
- github.com/pathToYourRepo</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Version">
Version
</a>
</em>
</td>
<td>
<p>Component versions refer to specific snapshots of a component. A common scenario being the release of a component.</p>
</td>
</tr>
<tr>
<td>
<code>repository</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Repository">
Repository
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>verify</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Signature">
[]Signature
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>references</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ReferencesConfig">
ReferencesConfig
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this component version object.</p>
</td>
</tr>
<tr>
<td>
<code>serviceAccountName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ServiceAccountName can be used to configure access to both destination and source repositories.
If service account is defined, it&rsquo;s usually redundant to define access to either source or destination, but
it is still allowed to do so.
<a href="https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account">https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account</a></p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ComponentVersionStatus">ComponentVersionStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersion">ComponentVersion</a>)
</p>
<p>ComponentVersionStatus defines the observed state of ComponentVersion</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>componentDescriptor</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Reference">
Reference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>reconciledVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>verified</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Configuration">Configuration
</h3>
<p>Configuration is the Schema for the configurations API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">
MutationSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>configRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>outputTemplate</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">
SnapshotTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>values</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON">
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>valuesFrom</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ValuesSource">
ValuesSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>patchStrategicMerge</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMerge">
PatchStrategicMerge
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.MutationStatus">
MutationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.FluxDeployer">FluxDeployer
</h3>
<p>FluxDeployer is the Schema for the fluxdeployers API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployerSpec">
FluxDeployerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kustomizationTemplate</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/kustomize-controller/api/v1beta2#KustomizationSpec">
github.com/fluxcd/kustomize-controller/api/v1beta2.KustomizationSpec
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployerStatus">
FluxDeployerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.FluxDeployerSpec">FluxDeployerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployer">FluxDeployer</a>)
</p>
<p>FluxDeployerSpec defines the desired state of FluxDeployer</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>kustomizationTemplate</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/kustomize-controller/api/v1beta2#KustomizationSpec">
github.com/fluxcd/kustomize-controller/api/v1beta2.KustomizationSpec
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.FluxDeployerStatus">FluxDeployerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployer">FluxDeployer</a>)
</p>
<p>FluxDeployerStatus defines the observed state of FluxDeployer</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>kustomization</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>ociRepository</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.FluxValuesSource">FluxValuesSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ValuesSource">ValuesSource</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#NamespacedObjectKindReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectKindReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>path</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Localization">Localization
</h3>
<p>Localization is the Schema for the localizations API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">
MutationSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>configRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>outputTemplate</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">
SnapshotTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>values</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON">
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>valuesFrom</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ValuesSource">
ValuesSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>patchStrategicMerge</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMerge">
PatchStrategicMerge
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.MutationStatus">
MutationStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.MutationObject">MutationObject
</h3>
<p>MutationObject defines any object which produces a snapshot</p>
<h3 id="delivery.ocm.software/v1alpha1.MutationSpec">MutationSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Configuration">Configuration</a>, 
<a href="#delivery.ocm.software/v1alpha1.Localization">Localization</a>)
</p>
<p>MutationSpec defines a common spec between Localization and Configuration.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>configRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>outputTemplate</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">
SnapshotTemplateSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>values</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1#JSON">
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>valuesFrom</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ValuesSource">
ValuesSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>patchStrategicMerge</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMerge">
PatchStrategicMerge
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.MutationStatus">MutationStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Configuration">Configuration</a>, 
<a href="#delivery.ocm.software/v1alpha1.Localization">Localization</a>)
</p>
<p>MutationStatus defines a common status for Localizations and Configurations.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>latestSnapshotDigest</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>latestSourceVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>latestConfigVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>latestPatchSourceVersio</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>snapshotName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ObjectReference">ObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployerSpec">FluxDeployerSpec</a>, 
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">MutationSpec</a>, 
<a href="#delivery.ocm.software/v1alpha1.ResourceSpec">ResourceSpec</a>)
</p>
<p>ObjectReference defines a resource which may be accessed via a snapshot or component version</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>NamespacedObjectKindReference</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#NamespacedObjectKindReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectKindReference
</a>
</em>
</td>
<td>
<p>
(Members of <code>NamespacedObjectKindReference</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>resourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ResourceReference">
ResourceReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.PatchStrategicMerge">PatchStrategicMerge
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">MutationSpec</a>)
</p>
<p>PatchStrategicMerge contains the source and target details required to perform a strategic merge</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>source</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMergeSource">
PatchStrategicMergeSource
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>target</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMergeTarget">
PatchStrategicMergeTarget
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.PatchStrategicMergeSource">PatchStrategicMergeSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMerge">PatchStrategicMerge</a>)
</p>
<p>PatchStrategicMergeSource contains the details required to retrieve the source from a Flux source</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#NamespacedObjectKindReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectKindReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>path</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.PatchStrategicMergeTarget">PatchStrategicMergeTarget
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.PatchStrategicMerge">PatchStrategicMerge</a>)
</p>
<p>PatchStrategicMergeTarget provides details about the merge target</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Reference">Reference
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionStatus">ComponentVersionStatus</a>, 
<a href="#delivery.ocm.software/v1alpha1.Reference">Reference</a>)
</p>
<p>Reference contains all referred components and their versions.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>references</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.Reference">
[]Reference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>extraIdentity</code><br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>componentDescriptorRef</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#NamespacedObjectReference">
github.com/fluxcd/pkg/apis/meta.NamespacedObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ReferencesConfig">ReferencesConfig
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>expand</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Repository">Repository
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<p>Repository defines the OCM Repository.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>url</code><br>
<em>
string
</em>
</td>
<td>
<p>TODO@souleb: do we need a scheme for the url?
add description for each field
Do we need a type field? (e.g. oci, git, s3, etc.)</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Resource">Resource
</h3>
<p>Resource is the Schema for the resources API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ResourceSpec">
ResourceSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
<p>SourceRef defines the input source from which the resource
will be retrieved</p>
</td>
</tr>
<tr>
<td>
<code>outputTemplate</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">
SnapshotTemplateSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ResourceStatus">
ResourceStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ResourceReference">ResourceReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">ObjectReference</a>)
</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ElementMeta</code><br>
<em>
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1.ElementMeta
</em>
</td>
<td>
<p>
(Members of <code>ElementMeta</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>referencePath</code><br>
<em>
[]github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Identity
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ResourceSpec">ResourceSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Resource">Resource</a>)
</p>
<p>ResourceSpec defines the desired state of Resource</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourceRef</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ObjectReference">
ObjectReference
</a>
</em>
</td>
<td>
<p>SourceRef defines the input source from which the resource
will be retrieved</p>
</td>
</tr>
<tr>
<td>
<code>outputTemplate</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">
SnapshotTemplateSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ResourceStatus">ResourceStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Resource">Resource</a>)
</p>
<p>ResourceStatus defines the observed state of Resource</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastAppliedResourceVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>lastAppliedComponentVersion</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastAppliedComponentVersion tracks the last applied component version. If there is a change
we fire off a reconcile loop to get that new version.</p>
</td>
</tr>
<tr>
<td>
<code>snapshotName</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>latestSnapshotDigest</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.SecretRefValue">SecretRefValue
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Signature">Signature</a>)
</p>
<p>SecretRefValue clearly denotes that the requested option is a Secret.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretRef</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Signature">Signature
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<p>Signature defines the details of a signature to use for verification.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
<p>Name of the signature.</p>
</td>
</tr>
<tr>
<td>
<code>publicKey</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SecretRefValue">
SecretRefValue
</a>
</em>
</td>
<td>
<p>Key which is used for verification.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Snapshot">Snapshot
</h3>
<p>Snapshot is the Schema for the snapshots API</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotSpec">
SnapshotSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>identity</code><br>
<em>
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Identity
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>digest</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tag</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.SnapshotStatus">
SnapshotStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.SnapshotSpec">SnapshotSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Snapshot">Snapshot</a>)
</p>
<p>SnapshotSpec defines the desired state of Snapshot</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>identity</code><br>
<em>
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Identity
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>digest</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tag</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Suspend stops all operations on this object.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.SnapshotStatus">SnapshotStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Snapshot">Snapshot</a>)
</p>
<p>SnapshotStatus defines the observed state of Snapshot</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Condition">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>digest</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Digest is calculated by the caching layer.</p>
</td>
</tr>
<tr>
<td>
<code>tag</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tag defines the explicit tag that was used to create the related snapshot and cache entry.</p>
</td>
</tr>
<tr>
<td>
<code>repositoryURL</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>RepositoryURL has the concrete URL pointing to the local registry including the service name.</p>
</td>
</tr>
<tr>
<td>
<code>observedGeneration</code><br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>ObservedGeneration is the last reconciled generation.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.SnapshotTemplateSpec">SnapshotTemplateSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">MutationSpec</a>, 
<a href="#delivery.ocm.software/v1alpha1.ResourceSpec">ResourceSpec</a>)
</p>
<p>SnapshotTemplateSpec defines the template used to create snapshots</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.SnapshotWriter">SnapshotWriter
</h3>
<p>SnapshotWriter defines any object which produces a snapshot</p>
<h3 id="delivery.ocm.software/v1alpha1.ValuesSource">ValuesSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.MutationSpec">MutationSpec</a>)
</p>
<p>ValuesSource provides access to values from an external Source such as a ConfigMap or GitRepository.
An optional subpath defines the path within the source from which the values should be resolved.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>fluxSource</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.FluxValuesSource">
FluxValuesSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Version">Version
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ComponentVersionSpec">ComponentVersionSpec</a>)
</p>
<p>Version defines version upgrade / downgrade options.</p>
<div class="md-typeset__scrollwrap">
<div class="md-typeset__table">
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>semver</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
