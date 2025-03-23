package reconciler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"tailscale.com/client/tailscale/v2"

	tsutils "github.com/chezmoi-sh/argotails/internal/tailscale"

	"testing"
)

const managedBy = "_"

type ReconcilerSuite struct {
	suite.Suite

	tailscaleMock  http.HandlerFunc
	kubernetesMock client.Client
	reconciler     *reconciler

	testserver *httptest.Server
}

func (suite *ReconcilerSuite) TestReconcile_NewDevice() {
	suite.tailscaleMock = func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := json.Marshal(map[string]any{
			"devices": []tailscale.Device{
				{
					Name:          "A.fake.ts.net",
					NodeID:        "fake-device-id",
					OS:            "linux",
					ClientVersion: "v1.2.3",
					Tags:          []string{"tag:tag1", "tag:tag2"},
				},
			},
		})

		_, _ = w.Write(raw)
	}

	res, err := suite.reconciler.Reconcile(
		context.TODO(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}},
	)
	suite.Require().NoError(err)
	suite.Equal(reconcile.Result{}, res)

	// Check the device secret freshly created.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Require().NoError(err)

	suite.Equal("fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("fake.ts.net", secret.Annotations[AnnotationDeviceTailnet])
	suite.Equal("cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	suite.Equal(managedBy, secret.Labels["apps.kubernetes.io/managed-by"])
	suite.Equal("linux", secret.Labels[LabelDeviceOS])
	suite.Equal("v1.2.3", secret.Labels[LabelDeviceVersion])
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag1")
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag2")
	suite.Equal("A.fake.ts.net", secret.StringData["name"])
	suite.Equal("https://A.fake.ts.net", secret.StringData["server"])
	suite.Equal(`{"tlsClientConfig":{"insecure":false}}`, secret.StringData["config"])
}

func (suite *ReconcilerSuite) TestReconcile_ExistingDevice() {
	// Create a new device secret.
	err := suite.kubernetesMock.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "A.fake.ts.net",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationDeviceID:      "fake-device-id",
				AnnotationDeviceTailnet: "fake.ts.net",
			},
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"apps.kubernetes.io/managed-by":  managedBy,
				LabelDeviceOS:                    "linux",
				LabelDeviceVersion:               "v1.2.3",
				LabelDeviceTagsPrefix + "tag1":   "",
				LabelDeviceTagsPrefix + "tag2":   "",
			},
		},
	})
	suite.Require().NoError(err)

	// Update the device secret.
	suite.tailscaleMock = func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := json.Marshal(map[string]any{
			"devices": []tailscale.Device{
				{
					Name:          "A.fake.ts.net",
					NodeID:        "fake-device-id",
					OS:            "linux",
					ClientVersion: "v1.2.3",
					Tags:          []string{"tag:tag1", "tag:tag2"},
				},
			},
		})

		_, _ = w.Write(raw)
	}

	res, err := suite.reconciler.Reconcile(
		context.TODO(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}},
	)
	suite.Require().NoError(err)
	suite.Equal(reconcile.Result{}, res)

	// Check the device secret freshly created.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Require().NoError(err)

	suite.Equal("fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("fake.ts.net", secret.Annotations[AnnotationDeviceTailnet])
	suite.Equal("cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	suite.Equal(managedBy, secret.Labels["apps.kubernetes.io/managed-by"])
	suite.Equal("linux", secret.Labels[LabelDeviceOS])
	suite.Equal("v1.2.3", secret.Labels[LabelDeviceVersion])
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag1")
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag2")
	suite.Equal("A.fake.ts.net", secret.StringData["name"])
	suite.Equal("https://A.fake.ts.net", secret.StringData["server"])
	suite.Equal(`{"tlsClientConfig":{"insecure":false}}`, secret.StringData["config"])
}

func (suite *ReconcilerSuite) TestReconcile_UpdatedExistingDevice() {
	// Create a new device secret.
	err := suite.kubernetesMock.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "A.fake.ts.net",
			Namespace: "default",
			Annotations: map[string]string{
				"existing-annotation": "true",
				AnnotationDeviceID:    "not-fake-device-id",
			},
			Labels: map[string]string{
				"existing-label": "true",
				LabelDeviceOS:    "not-linux",
			},
		},
	})
	suite.Require().NoError(err)

	// Update the device secret.
	suite.tailscaleMock = func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := json.Marshal(map[string]any{
			"devices": []tailscale.Device{
				{
					Name:          "A.fake.ts.net",
					NodeID:        "fake-device-id",
					OS:            "linux",
					ClientVersion: "v1.2.3",
					Tags:          []string{"tag:tag1", "tag:tag2"},
				},
			},
		})

		_, _ = w.Write(raw)
	}

	res, err := suite.reconciler.Reconcile(
		context.TODO(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}},
	)
	suite.Require().NoError(err)
	suite.Equal(reconcile.Result{}, res)

	// Check the device secret freshly created.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Require().NoError(err)

	// Note: The existing annotation and label are preserved if they are not one of the device metadata.
	suite.Equal("true", secret.Annotations["existing-annotation"])
	suite.NotEqual("not-fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("true", secret.Labels["existing-label"])
	suite.NotEqual("not-linux", secret.Labels[LabelDeviceOS])

	suite.Equal("fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("fake.ts.net", secret.Annotations[AnnotationDeviceTailnet])
	suite.Equal("cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	suite.Equal(managedBy, secret.Labels["apps.kubernetes.io/managed-by"])
	suite.Equal("linux", secret.Labels[LabelDeviceOS])
	suite.Equal("v1.2.3", secret.Labels[LabelDeviceVersion])
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag1")
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag2")
	suite.Equal("A.fake.ts.net", secret.StringData["name"])
	suite.Equal("https://A.fake.ts.net", secret.StringData["server"])
	suite.Equal(`{"tlsClientConfig":{"insecure":false}}`, secret.StringData["config"])
}

func (suite *ReconcilerSuite) TestReconcile_DeletedDevice() {
	// Create a new device secret.
	err := suite.kubernetesMock.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "A.fake.ts.net",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
	})
	suite.Require().NoError(err)

	// Update the device secret.
	suite.tailscaleMock = func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := json.Marshal(map[string]any{
			"devices": []tailscale.Device{},
		})

		_, _ = w.Write(raw)
	}

	res, err := suite.reconciler.Reconcile(
		context.TODO(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}},
	)
	suite.Require().NoError(err)
	suite.Equal(reconcile.Result{}, res)

	// Check the device secret freshly created.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Error(err)
	suite.True(errors.IsNotFound(err))
}

func (suite *ReconcilerSuite) TestReconcile_DeleteNonExistingDevice() {
	// Update the device secret.
	suite.tailscaleMock = func(w http.ResponseWriter, _ *http.Request) {
		raw, _ := json.Marshal(map[string]any{
			"devices": []tailscale.Device{},
		})

		_, _ = w.Write(raw)
	}

	res, err := suite.reconciler.Reconcile(
		context.TODO(),
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}},
	)
	suite.Require().NoError(err)
	suite.Equal(reconcile.Result{}, res)

	// Check the device secret freshly created.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Error(err)
	suite.True(errors.IsNotFound(err))
}

func (suite *ReconcilerSuite) TestCreateSecretDevice() {
	// Create a new device secret.
	err := suite.reconciler.CreateDeviceSecret(
		context.TODO(),
		types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"},
		tailscale.Device{
			Name:          "A.fake.ts.net",
			NodeID:        "fake-device-id",
			OS:            "linux",
			ClientVersion: "v1.2.3",
			Tags:          []string{"tag:tag1", "tag:tag2"},
		},
	)
	suite.Require().NoError(err)

	// Check the device secret.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Require().NoError(err)

	suite.Equal("fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("fake.ts.net", secret.Annotations[AnnotationDeviceTailnet])
	suite.Equal("cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	suite.Equal(managedBy, secret.Labels["apps.kubernetes.io/managed-by"])
	suite.Equal("linux", secret.Labels[LabelDeviceOS])
	suite.Equal("v1.2.3", secret.Labels[LabelDeviceVersion])
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag1")
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag2")
	suite.Equal("A.fake.ts.net", secret.StringData["name"])
	suite.Equal("https://A.fake.ts.net", secret.StringData["server"])
	suite.Equal(`{"tlsClientConfig":{"insecure":false}}`, secret.StringData["config"])
}

func (suite *ReconcilerSuite) TestUpdateSecretDevice() {
	// Create a new device secret.
	err := suite.kubernetesMock.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "A.fake.ts.net",
			Namespace: "default",
			Annotations: map[string]string{
				"existing-annotation": "true",
				AnnotationDeviceID:    "initial-device-id",
			},
			Labels: map[string]string{
				"existing-label": "true",
				LabelDeviceOS:    "initial-device-os",
			},
		},
		StringData: map[string]string{"existing-key": "existing-value"},
		Data:       map[string][]byte{"existing-binary-key": []byte("existing-binary-value")},
	})
	suite.Require().NoError(err)

	// Update the device secret.
	err = suite.reconciler.UpdateDeviceSecret(
		context.TODO(),
		types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"},
		tailscale.Device{
			Name:          "A.fake.ts.net",
			NodeID:        "fake-device-id",
			OS:            "linux",
			ClientVersion: "v1.2.3",
			Tags:          []string{"tag:tag1", "tag:tag2"},
		},
	)
	suite.Require().NoError(err)

	// Check the device secret.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Require().NoError(err)

	// Note: The existing annotation and label are preserved if they are not one of the device metadata.
	suite.Equal("true", secret.Annotations["existing-annotation"])
	suite.NotEqual("initial-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("true", secret.Labels["existing-label"])
	suite.NotEqual("initial-device-os", secret.Labels[LabelDeviceOS])

	suite.Equal("fake-device-id", secret.Annotations[AnnotationDeviceID])
	suite.Equal("fake.ts.net", secret.Annotations[AnnotationDeviceTailnet])
	suite.Equal("cluster", secret.Labels["argocd.argoproj.io/secret-type"])
	suite.Equal(managedBy, secret.Labels["apps.kubernetes.io/managed-by"])
	suite.Equal("linux", secret.Labels[LabelDeviceOS])
	suite.Equal("v1.2.3", secret.Labels[LabelDeviceVersion])
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag1")
	suite.Contains(secret.Labels, LabelDeviceTagsPrefix+"tag2")
	suite.Equal("A.fake.ts.net", secret.StringData["name"])
	suite.Equal("https://A.fake.ts.net", secret.StringData["server"])
	suite.Equal(`{"tlsClientConfig":{"insecure":false}}`, secret.StringData["config"])
}

func (suite *ReconcilerSuite) TestDeleteSecretDevice() {
	// Create a new device secret.
	err := suite.kubernetesMock.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "A.fake.ts.net",
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationDeviceID: "fake-device-id",
			},
		},
	})
	suite.Require().NoError(err)

	// Delete the device secret.
	err = suite.reconciler.DeleteDeviceSecret(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"})
	suite.Require().NoError(err)

	// Check the device secret.
	var secret corev1.Secret
	err = suite.kubernetesMock.Get(context.TODO(), types.NamespacedName{Name: "A.fake.ts.net", Namespace: "default"}, &secret)
	suite.Error(err)
	suite.True(errors.IsNotFound(err))
}

func (suite *ReconcilerSuite) SetupTest() {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	suite.Require().NoError(err)
	ks := fake.NewClientBuilder().WithScheme(scheme).Build()

	suite.testserver = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		suite.tailscaleMock(w, r)
	}))
	srvURL, err := url.Parse(suite.testserver.URL)
	suite.Require().NoError(err)
	ts := &tailscale.Client{Tailnet: "fake.ts.net", HTTP: suite.testserver.Client(), BaseURL: srvURL}

	suite.kubernetesMock = ks
	suite.tailscaleMock = func(w http.ResponseWriter, r *http.Request) { suite.Fail("request not mocked") }
	suite.reconciler = &reconciler{ts: ts, ks: ks, filter: tsutils.FuncTagFilter(func(_ tailscale.Device) bool { return true }), managedBy: managedBy}
}

func (suite *ReconcilerSuite) TearDownTest() {
	suite.testserver.Close()
	suite.reconciler = nil
}

func TestReconcilerSuite(t *testing.T) {
	suite.Run(t, new(ReconcilerSuite))
}
