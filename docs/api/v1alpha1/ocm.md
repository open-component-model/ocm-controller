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
<h3 id="delivery.ocm.software/v1alpha1.ComponentVersion">ComponentVersion
</h3>
<p>ComponentVersion is the Schema for the ComponentVersions API.</p>
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
<code>component</code><br>
<em>
string
</em>
</td>
<td>
<p>Component specifies the name of the ComponentVersion.</p>
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
<p>Version specifies the version information for the ComponentVersion.</p>
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
<p>Repository provides details about the OCI repository from which the component
descriptor can be retrieved.</p>
</td>
</tr>
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
<p>Interval specifies the interval at which the Repository will be checked for updates.</p>
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
<p>Verify specifies a list signatures that should be validated before the ComponentVersion
is marked Verified.</p>
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
<p>References specifies configuration for the handling of nested component references.</p>
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
<p>Suspend can be used to temporarily pause the reconciliation of the ComponentVersion resource.</p>
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
<p>ComponentVersionSpec specifies the configuration required to retrieve a
component descriptor for a component version.</p>
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
<code>component</code><br>
<em>
string
</em>
</td>
<td>
<p>Component specifies the name of the ComponentVersion.</p>
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
<p>Version specifies the version information for the ComponentVersion.</p>
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
<p>Repository provides details about the OCI repository from which the component
descriptor can be retrieved.</p>
</td>
</tr>
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
<p>Interval specifies the interval at which the Repository will be checked for updates.</p>
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
<p>Verify specifies a list signatures that should be validated before the ComponentVersion
is marked Verified.</p>
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
<p>References specifies configuration for the handling of nested component references.</p>
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
<p>Suspend can be used to temporarily pause the reconciliation of the ComponentVersion resource.</p>
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
<p>ComponentVersionStatus defines the observed state of ComponentVersion.</p>
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
<p>Conditions holds the conditions for the ComponentVersion.</p>
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
<p>ComponentDescriptor holds the ComponentDescriptor information for the ComponentVersion.</p>
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
<p>ReconciledVersion is a string containing the version of the latest reconciled ComponentVersion.</p>
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
<p>Verified is a boolean indicating whether all the specified signatures have been verified and are valid.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.ConfigMapSource">ConfigMapSource
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
<a href="https://pkg.go.dev/github.com/fluxcd/pkg/apis/meta#LocalObjectReference">
github.com/fluxcd/pkg/apis/meta.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>key</code><br>
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
<h3 id="delivery.ocm.software/v1alpha1.Configuration">Configuration
</h3>
<p>Configuration is the Schema for the configurations API.</p>
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
<h3 id="delivery.ocm.software/v1alpha1.DeliverySpec">DeliverySpec
</h3>
<p>DeliverySpec holds a set of targets onto which the pipeline output will be deployed.</p>
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
<code>targets</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.WasmStep">
[]WasmStep
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
<h3 id="delivery.ocm.software/v1alpha1.ElementMeta">ElementMeta
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ResourceReference">ResourceReference</a>)
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
<code>extraIdentity</code><br>
<em>
<a href="https://pkg.go.dev/github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1#Identity">
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Identity
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br>
<em>
<a href="https://pkg.go.dev/github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1#Labels">
github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Labels
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
<p>FluxDeployer is the Schema for the FluxDeployers API.</p>
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
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>The interval at which to reconcile the Kustomization and Helm Releases.</p>
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
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>helmReleaseTemplate</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/helm-controller/api/v2betapkg1#HelmReleaseSpec">
github.com/fluxcd/helm-controller/api/v2beta1.HelmReleaseSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
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
<p>FluxDeployerSpec defines the desired state of FluxDeployer.</p>
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
<code>interval</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<p>The interval at which to reconcile the Kustomization and Helm Releases.</p>
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
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>helmReleaseTemplate</code><br>
<em>
<a href="https://pkg.go.dev/github.com/fluxcd/helm-controller/api/v2betapkg1#HelmReleaseSpec">
github.com/fluxcd/helm-controller/api/v2beta1.HelmReleaseSpec
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
<h3 id="delivery.ocm.software/v1alpha1.FluxDeployerStatus">FluxDeployerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.FluxDeployer">FluxDeployer</a>)
</p>
<p>FluxDeployerStatus defines the observed state of FluxDeployer.</p>
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
<p>Localization is the Schema for the localizations API.</p>
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
<p>MutationSpec defines a common spec for Localization and Configuration of OCM resources.</p>
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
<a href="#delivery.ocm.software/v1alpha1.ResourcePipelineSpec">ResourcePipelineSpec</a>, 
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
<p>PatchStrategicMerge contains the source and target details required to perform a strategic merge.</p>
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
<p>PatchStrategicMergeSource contains the details required to retrieve the source from a Flux source.</p>
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
<p>PatchStrategicMergeTarget provides details about the merge target.</p>
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
<h3 id="delivery.ocm.software/v1alpha1.PipelineSpec">PipelineSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ResourcePipelineSpec">ResourcePipelineSpec</a>)
</p>
<p>PipelineSpec holds the steps that constitute the pipeline.</p>
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
<code>steps</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.WasmStep">
[]WasmStep
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
<h3 id="delivery.ocm.software/v1alpha1.PublicKey">PublicKey
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Signature">Signature</a>)
</p>
<p>PublicKey specifies access to a public key for verification.</p>
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
<em>(Optional)</em>
<p>SecretRef is a reference to a Secret that contains a public key.</p>
</td>
</tr>
<tr>
<td>
<code>value</code><br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Value defines a PEM/base64 encoded public key value.</p>
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
<p>Name specifies the name of the referenced component.</p>
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
<p>Version specifies the version of the referenced component.</p>
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
<p>References is a list of component references.</p>
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
<p>ExtraIdentity specifies additional identity attributes of the referenced component.</p>
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
<p>ComponentDescriptorRef specifies the reference for the Kubernetes object representing
the ComponentDescriptor.</p>
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
<p>ReferencesConfig specifies how component references should be handled when reconciling
the root component.</p>
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
<p>Expand specifies if a Kubernetes API resource of kind ComponentDescriptor should
be generated for each component reference that is present in the root ComponentVersion.</p>
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
<p>Repository specifies access details for the repository that contains OCM ComponentVersions.</p>
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
<p>URL specifies the URL of the OCI registry in which the ComponentVersion is stored.
MUST NOT CONTAIN THE SCHEME.</p>
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
<p>SecretRef specifies the credentials used to access the OCI registry.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.Resource">Resource
</h3>
<p>Resource is the Schema for the resources API.</p>
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
<p>Interval specifies the interval at which the Repository will be checked for updates.</p>
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
<p>SourceRef specifies the source object from which the resource should be retrieved.</p>
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
<p>Suspend can be used to temporarily pause the reconciliation of the Resource.</p>
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
<h3 id="delivery.ocm.software/v1alpha1.ResourcePipeline">ResourcePipeline
</h3>
<p>ResourcePipeline is the Schema for the resourcepipelines API.</p>
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
<a href="#delivery.ocm.software/v1alpha1.ResourcePipelineSpec">
ResourcePipelineSpec
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
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
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
<code>parameters</code><br>
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
<code>pipelineSpec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PipelineSpec">
PipelineSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ResourcePipelineStatus">
ResourcePipelineStatus
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
<h3 id="delivery.ocm.software/v1alpha1.ResourcePipelineSource">ResourcePipelineSource
</h3>
<p>ResourcePipelineSource defines the component version and resource
which will be processed by the pipeline.</p>
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
<code>namespace</code><br>
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
<code>resource</code><br>
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
<h3 id="delivery.ocm.software/v1alpha1.ResourcePipelineSpec">ResourcePipelineSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ResourcePipeline">ResourcePipeline</a>)
</p>
<p>ResourcePipelineSpec defines the desired state of ResourcePipeline.</p>
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
<code>suspend</code><br>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
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
<code>parameters</code><br>
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
<code>pipelineSpec</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PipelineSpec">
PipelineSpec
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
<h3 id="delivery.ocm.software/v1alpha1.ResourcePipelineStatus">ResourcePipelineStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.ResourcePipeline">ResourcePipeline</a>)
</p>
<p>ResourcePipelineStatus defines the observed state of ResourcePipeline.</p>
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
<a href="#delivery.ocm.software/v1alpha1.ElementMeta">
ElementMeta
</a>
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
<a href="https://pkg.go.dev/github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1#Identity">
[]github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1.Identity
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
<h3 id="delivery.ocm.software/v1alpha1.ResourceSpec">ResourceSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.Resource">Resource</a>)
</p>
<p>ResourceSpec defines the desired state of Resource.</p>
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
<p>Interval specifies the interval at which the Repository will be checked for updates.</p>
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
<p>SourceRef specifies the source object from which the resource should be retrieved.</p>
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
<p>Suspend can be used to temporarily pause the reconciliation of the Resource.</p>
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
<p>ResourceStatus defines the observed state of Resource.</p>
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
<p>Conditions holds the conditions for the ComponentVersion.</p>
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
<p>LastAppliedResourceVersion holds the version of the resource that was last applied (if applicable).</p>
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
<p>LastAppliedComponentVersion holds the version of the last applied ComponentVersion for the ComponentVersion which contains this Resource.</p>
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
<p>SnapshotName specifies the name of the Snapshot that has been created to store the resource
within the cluster and make it available for consumption by Flux controllers.</p>
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
<p>LatestSnapshotDigest is a string representation of the digest for the most recent Resource snapshot.</p>
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
<p>Name specifies the name of the signature. An OCM component may have multiple
signatures.</p>
</td>
</tr>
<tr>
<td>
<code>publicKey</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.PublicKey">
PublicKey
</a>
</em>
</td>
<td>
<p>PublicKey provides a reference to a Kubernetes Secret of contain a blob of a public key that
which will be used to validate the named signature.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
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
<tr>
<td>
<code>configMapSource</code><br>
<em>
<a href="#delivery.ocm.software/v1alpha1.ConfigMapSource">
ConfigMapSource
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
<p>Version specifies version information that can be used to resolve a Component Version.</p>
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
<p>Semver specifies a semantic version constraint for the Component Version.</p>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<h3 id="delivery.ocm.software/v1alpha1.WasmStep">WasmStep
</h3>
<p>
(<em>Appears on:</em>
<a href="#delivery.ocm.software/v1alpha1.DeliverySpec">DeliverySpec</a>, 
<a href="#delivery.ocm.software/v1alpha1.PipelineSpec">PipelineSpec</a>)
</p>
<p>WasmStep defines the name version and location of a wasm module that is stored// in an ocm component.
The format of the module name must be <component-name>:<component-version>@<resource-name>. Optionally a registry address can be specified.</p>
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
<code>module</code><br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>registry</code><br>
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
<code>timeout</code><br>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
Kubernetes meta/v1.Duration
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
<div class="admonition note">
<p class="last">This page was automatically generated with <code>gen-crd-api-reference-docs</code></p>
</div>
