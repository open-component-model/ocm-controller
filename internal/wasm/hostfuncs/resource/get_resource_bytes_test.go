package resource_test

// import (
//     "github.com/mandelsoft/vfs/pkg/projectionfs"
//     "github.com/open-component-model/ocm/pkg/common/accessio"
//     "github.com/open-component-model/ocm/pkg/common/accessobj"
//     metav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
//     "github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/ctf"
//     "github.com/open-component-model/ocm/pkg/contexts/ocm/resourcetypes"
//     "github.com/open-component-model/ocm/pkg/env"
//     "github.com/open-component-model/ocm/pkg/env/builder"
//     "github.com/open-component-model/ocm/pkg/mime"
//     "github.com/open-component-model/ocm/pkg/utils/tarutils"
// )
//
// var (
//     name    = "ocm.software/test-component"
//     version = "v0.0.0"
// )
//
// var _ = Describe("Get Resource Bytes", func() {
//     var testEnv *builder.Builder
//
//     BeforeEach(func() {
//         testEnv = builder.NewBuilder(env.NewEnvironment(env.TestData()))
//         fs, err := projectionfs.New(testEnv, "testdata/manifests")
//         Expect(err).To(Succeed())
//
//         err = tarutils.CreateTarFromFs(fs, "manifests.tgz", tarutils.Gzip, testEnv)
//         Expect(err).To(Succeed())
//
//         testEnv.OCMCommonTransport("/tmp/ctf", accessio.FormatDirectory, func() {
//             testEnv.Component(name, func() {
//                 testEnv.Version(version, func() {
//                     testEnv.Provider("github.com/open-component-model")
//                     testEnv.Resource("manifests", "", resourcetypes.DIRECTORY_TREE, metav1.LocalRelation, func() {
//                         testEnv.BlobFromFile(mime.MIME_TGZ_ALT, "manifests.tgz")
//                     })
//                 })
//             })
//         })
//     })
//
//     AfterEach(func() {
//         testEnv.Cleanup()
//     })
//
//     It("should get resources bytes", func() {
//         src, err := ctf.Open(testEnv.OCMContext(), accessobj.ACC_READONLY, "/tmp/ctf", 0, testEnv)
//         Expect(err).To(Succeed())
//
//         cv, err := src.LookupComponentVersion(name, version)
//         Expect(err).To(Succeed())
//
//         closure := getResourceBytes(cv)
//         Expect(closure).ToNot(BeNil())
//
//         // TODO: create simple wasm runtime and module for testing
//     })
// })
