package reconciler

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"tailscale.com/client/tailscale/v2"

	ts "github.com/chezmoi-sh/argotails/internal/tailscale"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AnnotationDeviceID is the annotation key for the device ID.
	AnnotationDeviceID = "device.tailscale.com/id"
	// AnnotationDeviceHostname is the annotation key for the device hostname.
	AnnotationDeviceHostname = "device.tailscale.com/hostname"
	// AnnotationDeviceTailnet is the annotation key for the device name.
	AnnotationDeviceTailnet = "device.tailscale.com/tailnet"

	// LabelDeviceOS is the label key for the device OS.
	LabelDeviceOS = "device.tailscale.com/os"
	// LabelDeviceVersion is the label key for the device version.
	LabelDeviceVersion = "device.tailscale.com/version"

	// LabelDeviceTagsPrefix is the label key prefix used for the device tags.
	LabelDeviceTagsPrefix = "tag.device.tailscale.com/"
)

// regex to extract the tailnet from the device name
var rxTailnet = regexp.MustCompile(`\.(.+\.ts\.net$)`)

type (
	reconciler struct {
		// ts is the Tailscale client.
		ts *tailscale.Client
		// ks is the Kubernetes client.
		ks client.Client

		// filter filters the devices based on their tags.
		filter ts.TagFilter
		// managedBy is the controller name.
		managedBy string
	}

	Config struct {
		// Tailnet is the Tailscale network name.
		Tailnet string
		// AuthKey is the Tailscale OAuth key.
		AuthKey string

		// DeviceFilters is the list of tag filters to apply to the devices.
		DeviceFilters []string
	}
)

// NewReconciler creates a new reconciler based on the provided configuration.
func NewReconciler(ks client.Client, ts *tailscale.Client, filter ts.TagFilter, managedBy string) (reconcile.TypedReconciler[reconcile.Request], error) {
	reconciler := &reconciler{ks: ks, ts: ts, filter: filter, managedBy: managedBy}
	return reconciler, nil
}

// Reconcile reconciles a secret with a Tailscale device by creating, updating or deleting the secret
// based on the device's existence and metadata.
func (r reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := ctrllog.FromContext(ctx).WithName("reconcile_secret")
	log.V(0).Info("Starting reconciliation of Tailscale device's secret")

	log.V(2).Info("Listing Tailscale devices")
	devices, err := r.ts.Devices().List(ctx)
	if err != nil {
		log.Error(err, "Failed to list Tailscale devices", "reconciliation.outcome", "tailscale_list_error")
		return reconcile.Result{Requeue: true}, fmt.Errorf("failed to list devices: %w", err)
	}
	log.V(4).Info("Tailscale API call completed", "devices.count", len(devices))

	var device *tailscale.Device
	for _, _device := range devices {
		if _device.Name == req.Name {
			device = &_device
			log = log.WithValues("device", map[string]any{
				"id":       _device.NodeID,
				"name":     _device.Name,
				"os":       _device.OS,
				"hostname": _device.Hostname,
				"version":  _device.ClientVersion,
			})
			log.V(2).Info("Found matching Tailscale device")
			break
		}
	}

	if device == nil || !r.filter.Match(*device) {
		log.V(0).Info("Tailscale device not found or filtered, Tailscale device's secret will be deleted", "reconciliation.action", "delete")
		err := r.DeleteDeviceSecret(ctrllog.IntoContext(ctx, log), req.NamespacedName)
		if err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete Tailscale device's secret", "reconciliation.outcome", "delete_secret_error")
			return reconcile.Result{Requeue: true}, err
		}
		log.V(1).Info("Device reconciliation completed with deletion", "reconciliation.outcome", "deleted")
		return reconcile.Result{}, nil
	}

	var secret corev1.Secret
	err = r.ks.Get(ctx, req.NamespacedName, &secret)
	if errors.IsNotFound(err) {
		log.V(1).Info("Tailscale device's secret not found, Tailscale device's secret will be created", "reconciliation.action", "create")
		err = r.CreateDeviceSecret(ctrllog.IntoContext(ctx, log), req.NamespacedName, *device)
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Error(err, "Failed to create Tailscale device's secret", "reconciliation.outcome", "create_secret_error")
			return reconcile.Result{Requeue: true}, err
		}
		log.V(1).Info("Device reconciliation completed with creation", "reconciliation.outcome", "created")
		return reconcile.Result{}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Tailscale device's secret", "reconciliation.outcome", "get_secret_error")
		return reconcile.Result{Requeue: true}, err
	}

	log.V(2).Info("Tailscale device's secret found, Tailscale device's secret will be updated", "reconciliation.action", "update")
	err = r.UpdateDeviceSecret(ctrllog.IntoContext(ctx, log), req.NamespacedName, *device)
	if err != nil {
		log.Error(err, "Failed to update Tailscale device's secret", "reconciliation.outcome", "update_secret_error")
		return reconcile.Result{Requeue: true}, err
	}

	log.V(1).Info("Device reconciliation completed with update", "reconciliation.outcome", "updated")
	return reconcile.Result{}, nil
}

// CreateDeviceSecret creates a new Tailscale device's secret based on the device's metadata.
func (r reconciler) CreateDeviceSecret(ctx context.Context, namespacedName types.NamespacedName, device tailscale.Device) error {
	log := ctrllog.FromContext(ctx).WithName("create")

	tailnet := ""
	if rxTailnet.MatchString(device.Name) {
		tailnet = rxTailnet.FindStringSubmatch(device.Name)[1]
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: "default",
			Annotations: map[string]string{
				AnnotationDeviceID:       device.NodeID,
				AnnotationDeviceHostname: device.Hostname,
			},
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"apps.kubernetes.io/managed-by":  r.managedBy,

				LabelDeviceOS:      device.OS,
				LabelDeviceVersion: device.ClientVersion,
			},
		},
		StringData: map[string]string{
			"name":   device.Name,
			"server": fmt.Sprintf("https://%s", device.Name),
			"config": `{"tlsClientConfig":{"insecure":false}}`,
		},
	}

	if tailnet != "" {
		secret.Annotations[AnnotationDeviceTailnet] = tailnet
	}

	// Process device tags
	for _, tag := range device.Tags {
		secret.Labels[LabelDeviceTagsPrefix+strings.TrimPrefix(tag, "tag:")] = ""
	}

	log.V(3).Info("Create Tailscale device secret")
	return r.ks.Create(ctx, &secret)
}

// UpdateDeviceSecret updates an existing Tailscale device's secret based on the device's metadata.
func (r reconciler) UpdateDeviceSecret(ctx context.Context, namespacedName types.NamespacedName, device tailscale.Device) error {
	log := ctrllog.FromContext(ctx).WithName("update")

	log.V(3).Info("Retrieving current Tailscale device's secret")
	var secret corev1.Secret
	err := r.ks.Get(ctx, namespacedName, &secret)
	if err != nil {
		return err
	}

	// Update secret metadata
	secret.Annotations[AnnotationDeviceID] = device.NodeID
	secret.Annotations[AnnotationDeviceHostname] = device.Hostname
	secret.Labels["argocd.argoproj.io/secret-type"] = "cluster"
	secret.Labels["apps.kubernetes.io/managed-by"] = r.managedBy
	secret.Labels[LabelDeviceOS] = device.OS
	secret.Labels[LabelDeviceVersion] = device.ClientVersion

	tailnet := ""
	if rxTailnet.MatchString(device.Name) {
		tailnet = rxTailnet.FindStringSubmatch(device.Name)[1]
		secret.Annotations[AnnotationDeviceTailnet] = tailnet
	}

	// Process device tags
	for _, tag := range device.Tags {
		secret.Labels[LabelDeviceTagsPrefix+strings.TrimPrefix(tag, "tag:")] = ""
	}

	secret.Data = nil
	secret.StringData = map[string]string{
		"name":   device.Name,
		"server": fmt.Sprintf("https://%s", device.Name),
		"config": `{"tlsClientConfig":{"insecure":false}}`,
	}

	log.V(3).Info("Update Tailscale device secret")
	return r.ks.Update(ctx, &secret)
}

// DeleteDeviceSecret deletes an existing Tailscale device's secret.
func (r reconciler) DeleteDeviceSecret(ctx context.Context, namespacedName types.NamespacedName) error {
	log := ctrllog.FromContext(ctx).WithName("delete")

	// Get the secret first to check if it exists and log its metadata
	log.V(3).Info("Retrieving current Tailscale device's secret")
	var secret corev1.Secret
	err := r.ks.Get(ctx, namespacedName, &secret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Secret does not exist, nothing to do
			log.V(3).Info("Tailscale device's secret not found, ignoring")
			return nil
		}
		return err
	}

	log.V(3).Info("Delete Tailscale device secret")
	return r.ks.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
		},
	})
}

// KubernetesClient returns the Kubernetes client.
func (r reconciler) KubernetesClient() client.Client { return r.ks }

// TailscaleClient returns the Tailscale client.
func (r reconciler) TailscaleClient() *tailscale.Client { return r.ts }
