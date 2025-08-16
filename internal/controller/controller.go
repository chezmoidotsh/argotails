package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/oklog/ulid/v2"
	"github.com/prometheus/common/version"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"tailscale.com/client/tailscale/v2"

	"github.com/chezmoi-sh/argotails/internal/reconciler"
	tsutils "github.com/chezmoi-sh/argotails/internal/tailscale"
)

type (
	VersionCmd struct{}
	RunCmd     struct {
		ReconcileInterval time.Duration `name:"reconcile.interval" help:"Time between two Tailscale devices and ArgoCD cluster secrets reconciliation." default:"30s" env:"RECONCILE_INTERVAL"`

		Tailscale struct {
			BaseURL          *url.URL `name:"base-url" help:"Tailscale API base URL." default:"https://api.tailscale.com" env:"TAILSCALE_BASE_URL" group:"Tailscale flags"`
			Tailnet          string   `name:"tailnet" required:"" placeholder:"TAILSCALE_TAILNET" help:"Tailscale network name." env:"TAILSCALE_TAILNET" group:"Tailscale flags"`
			AuthKey          string   `name:"authkey" required:"" placeholder:"TAILSCALE_AUTH_KEY" help:"Tailscale OAuth key." env:"TAILSCALE_AUTH_KEY" group:"Tailscale flags" xor:"authkey"`
			AuthKeyFile      []byte   `name:"authkey-file" type:"filecontent" placeholder:"TAILSCALE_AUTH_KEY_FILE" help:"Path to the file containing the Tailscale OAuth key." env:"TAILSCALE_AUTH_KEY_FILE" group:"Tailscale flags" xor:"authkey"`
			DeviceTagFilters []string `name:"device-filter" placeholder:"PATTERN" help:"List of regular expressions to filter the Tailscale devices based on their tags." group:"Tailscale flags"`

			Webhook struct {
				Enable     bool   `name:"enable" help:"Enable the Tailscale webhook handler." default:"false" env:"TAILSCALE_WEBHOOK_ENABLE" group:"Tailscale flags"`
				Port       int    `name:"port" help:"Tailscale webhook port." default:"3000" env:"TAILSCALE_WEBHOOK_PORT" group:"Tailscale flags" `
				Secret     string `name:"secret" placeholder:"TAILSCALE_WEBHOOK_SECRET" help:"Tailscale webhook secret." env:"TAILSCALE_WEBHOOK_SECRET" group:"Tailscale flags" xor:"webhook"`
				SecretFile []byte `name:"secret-file"  type:"filecontent" placeholder:"TAILSCALE_WEBHOOK_SECRET_FILE" help:"Path to the file containing the Tailscale webhook secret." env:"TAILSCALE_WEBHOOK_SECRET_FILE" group:"Tailscale flags" xor:"webhook"`
			} `embed:"" prefix:"webhook."`
		} `embed:"" prefix:"ts."`

		Namespace string `name:"namespace" help:"Namespace where ArgoCD cluster secret must be created (configure it only if Argotails runs outside the cluster)." env:"NAMESPACE"` // trunk-ignore(golangci-lint/lll)

		Service struct {
			CreateService bool   `name:"create" help:"Create Kubernetes services with Tailscale annotations for multi-cluster ArgoCD support." default:"false" env:"CREATE_SERVICE" group:"Service flags"`
			ProxyClass    string `name:"proxy-class" help:"ProxyClass to use for Tailscale services (optional)." env:"SERVICE_PROXY_CLASS" group:"Service flags"`
		} `embed:"" prefix:"service." envprefix:"SERVICE_"`

		Log struct {
			Development bool                 `name:"devel" help:"Enable development logging." env:"DEVEL" group:"Log flags"`
			Verbosity   zapcore.LevelEnabler `name:"v" help:"Log verbosity level." default:"2" env:"VERBOSITY" group:"Log flags"`
			Format      zapcore.Encoder      `name:"format" help:"Log encoding format, either 'json' or 'console'." default:"json" env:"FORMAT" group:"Log flags"`
		} `embed:"" prefix:"log." envprefix:"LOG_"`

		ts         *tailscale.Client
		mgr        manager.Manager
		ctrlName   string
		reconciler reconcile.TypedReconciler[reconcile.Request]
	}

	Command struct {
		Run     RunCmd     `cmd:"" help:"Run the ArgoCD Tailscale integration controller."`
		Version VersionCmd `cmd:"" name:"version" help:"Show version information and exit."`
	}
)

func (VersionCmd) Run(cli *kong.Context) error {
	fmt.Println(version.Print(cli.Model.Name))
	return nil
}

func (c *RunCmd) AfterApply() error {
	if c.Tailscale.AuthKeyFile != nil {
		c.Tailscale.AuthKey = string(c.Tailscale.AuthKeyFile)
	}
	if c.Tailscale.Webhook.SecretFile != nil {
		c.Tailscale.Webhook.Secret = string(c.Tailscale.Webhook.SecretFile)
	}
	if c.Namespace == "" {
		ns, _ := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if len(ns) == 0 {
			return errors.New("--namespace is required when running outside a cluster or service account not mounted")
		}
		c.Namespace = string(ns)
	}
	return nil
}

func (c *RunCmd) Run(cli *kong.Context) error {
	c.ctrlName = cli.Model.Name

	// Initialize the logger and context
	zopts := []zap.Opts{
		zap.UseFlagOptions(&zap.Options{
			TimeEncoder: zapcore.ISO8601TimeEncoder,
		}),
		zap.Level(c.Log.Verbosity),
		zap.Encoder(c.Log.Format),
	}
	if c.Log.Development {
		zopts = []zap.Opts{zap.UseDevMode(true), zap.Level(zapcore.Level(-127))}
	}
	log := zap.New(zopts...)
	ctrllog.SetLogger(log)
	ctx := ctrllog.IntoContext(signals.SetupSignalHandler(), log)

	// Log startup configuration
	log.V(0).Info("Starting ArgoCD Tailscale integration controller", "version", version.Version)

	// Configure the Tailscale client.
	log.V(1).Info("Initializing Tailscale client",
		"tailscale", map[string]any{
			"baseURL": c.Tailscale.BaseURL.String(),
			"tailnet": c.Tailscale.Tailnet,
		},
	)
	var err error
	c.ts, err = tsutils.NewTailscaleClient(c.Tailscale.BaseURL, c.Tailscale.Tailnet, c.Tailscale.AuthKey)
	if err != nil {
		log.Error(err, "Unable to create Tailscale client. Please check the configuration and try again.")
		return err
	}
	log.V(1).Info("Tailscale client initialized successfully")

	// Configure the controller manager.
	log.V(1).Info("Initializing controller manager")
	c.mgr, err = manager.New(config.GetConfigOrDie(), manager.Options{
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				// Argotails controller must only watch secrets managed by itself inside the configured namespace
				// (or the namespace where it runs if it's running inside a Kubernetes cluster). This ensures that
				// the controller will not interfere with other controllers or resources and will not read secrets
				// from other namespaces.
				c.Namespace: {LabelSelector: labels.SelectorFromSet(labels.Set{"apps.kubernetes.io/managed-by": c.ctrlName})},
			},
		},
		HealthProbeBindAddress: ":8081", // Expose health endpoints
		BaseContext:            func() context.Context { return ctx },
		Logger:                 log,
	})
	if err != nil {
		log.Error(err, "Unable to set up the overall controller manager. Please check the configuration and try again.")
		return err
	}

	// Add health check endpoints
	if err := c.mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up health check", "error", err)
		return err
	}
	if err := c.mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up ready check", "error", err)
		return err
	}

	log.V(1).Info("Controller manager initialized successfully")

	// Configure the Kubernetes reconciler.
	log.V(1).Info("Initializing tag filter", "filter.patterns", c.Tailscale.DeviceTagFilters)
	filter, err := tsutils.NewRegexpTagFilter(c.Tailscale.DeviceTagFilters...)
	if err != nil {
		log.Error(err, "Invalid Tailscale devices' tag filters.", "filter.patterns", c.Tailscale.DeviceTagFilters)
		return err
	}
	log.V(1).Info("Tag filter initialized successfully")

	log.V(1).Info("Initializing reconciler")
	c.reconciler, err = reconciler.NewReconciler(c.mgr.GetClient(), c.ts, filter, c.ctrlName, reconciler.ServiceConfig{
		CreateService: c.Service.CreateService,
		ProxyClass:    c.Service.ProxyClass,
		Namespace:     c.Namespace,
	})
	if err != nil {
		log.Error(err, "Unable to create Tailscale reconciler. Please check the configuration and try again.")
		return err
	}
	log.V(1).Info("Reconciler initialized successfully")

	// Configure all reconciliation loops
	log.V(1).Info("Setting up reconciliation loops")
	errg, ctx := errgroup.WithContext(ctx)

	loopCtx := ctrllog.IntoContext(ctx, log.WithName("main"))
	errg.Go(func() error { return c.kubernetesReconcilationLoop(loopCtx) })
	errg.Go(func() error { return c.timeBasedReconciliationLoop(loopCtx, filter) })
	if c.Tailscale.Webhook.Enable {
		errg.Go(func() error { return c.webhookReconciliationLoop(loopCtx) })
	}

	// Start the controller
	log.V(0).Info("Controller initialization completed")
	return errg.Wait()
}

func (c *RunCmd) kubernetesReconcilationLoop(ctx context.Context) error {
	log := ctrllog.
		FromContext(ctx).
		WithName("kubernetes")
	log.V(1).Info("Initializing Kubernetes controller")

	controllerBuilder := builder.
		ControllerManagedBy(c.mgr).
		Named(c.ctrlName).
		For(&corev1.Secret{}).
		WithLogConstructor(func(r *reconcile.Request) logr.Logger {
			if r == nil {
				return log
			}
			return log.WithValues("resource", r)
		})

	// Also watch services if service creation is enabled
	if c.Service.CreateService {
		controllerBuilder = controllerBuilder.Owns(&corev1.Service{})
	}

	err := controllerBuilder.Complete(c.reconciler)

	if err != nil {
		log.Error(err, "Unable to create controller")
		return err
	}
	log.V(1).Info("Kubernetes controller initialized successfully, starting manager")

	if err = c.mgr.Start(ctx); err != nil {
		log.Error(err, "Failed to start the controller manager")
		return err
	}

	log.V(0).Info("Controller manager successfully stopped")
	return nil
}

func (c *RunCmd) timeBasedReconciliationLoop(ctx context.Context, filter tsutils.TagFilter) error {
	log := ctrllog.FromContext(ctx).WithName("time_based")
	log.V(1).Info("Starting time-based reconciliation loop")

	ticker := time.NewTicker(c.ReconcileInterval)
	defer ticker.Stop()

	// NOTE: in order to have a clean way to handle error on the reconciliation loop
	//		 we will use an anonymous function
	syncAllDevices := func(ctx context.Context) error {
		log.V(1).Info("Starting device synchronization")

		// All devices to reconcile will be stored in deviceToSync
		deviceToSync := map[reconcile.Request]any{}

		// Get all Tailscale devices
		log.V(2).Info("Listing all Tailscale devices")
		devices, err := c.ts.Devices().List(ctx)

		if err != nil {
			log.Error(err, "Failed to list Tailscale devices")
			return fmt.Errorf("failed to list Tailscale devices: %w", err)
		}
		log.V(3).Info("Retrieved Tailscale devices", "devices", map[string]any{"count": len(devices)})

		// Apply filter to devices
		for _, device := range devices {
			if filter.Match(device) {
				deviceToSync[reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      device.Name,
						Namespace: c.Namespace,
					},
				}] = struct{}{}
			} else {
				log.V(4).Info("Device ignored by filter",
					"device", map[string]any{
						"name": device.Name,
						"id":   device.NodeID,
						"tags": device.Tags,
					},
				)
			}
		}

		// Get all existing secrets managed by this controller
		log.V(2).Info("Listing existing Tailscale device secrets")
		existingSecrets := corev1.SecretList{}
		err = c.mgr.GetClient().List(
			ctx,
			&existingSecrets,
			client.InNamespace(c.Namespace),
			client.MatchingLabels{"apps.kubernetes.io/managed-by": c.ctrlName},
		)
		if err != nil {
			log.Error(err, "Failed to list existing Tailscale devices' secrets")
			return fmt.Errorf("failed to list existing Tailscale devices' secrets: %w", err)
		}

		// Add all existing secrets to reconciliation list
		for _, secret := range existingSecrets.Items {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}
			if _, exists := deviceToSync[req]; !exists {
				log.V(3).Info("Adding existing secret to sync list",
					"secret", map[string]any{
						"name":      secret.Name,
						"namespace": secret.Namespace,
					},
				)
				deviceToSync[req] = struct{}{}
			}
		}

		// Reconcile all devices
		log.V(1).Info("Starting reconciliation of all devices", "devices", map[string]any{"count": len(deviceToSync)})

		var errs *multierror.Error
		for req := range deviceToSync {
			log.V(3).Info("Reconciling device", "device", req)
			_, err := c.reconciler.Reconcile(ctrllog.IntoContext(ctx, log), req)

			if err != nil {
				log.Error(err, "Failed to reconcile device")
				errs = multierror.Append(errs, err)
			} else {
				log.V(3).Info("Successfully reconciled device")
			}
		}

		if err := errs.ErrorOrNil(); err != nil {
			log.Error(err, "Device synchronization completed with error")
		}
		log.V(0).Info("Device synchronization successfully completed")
		return nil
	}

	// Run a first reconciliation when the manager starts
	_ = c.mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		log.V(1).Info("Running initial Tailscale devices reconciliation")
		return syncAllDevices(ctx)
	}))

	remainingRetries := 5
	for {
		select {
		case <-ticker.C:
			log.V(1).Info("Reconciliation interval reached")

			if err := syncAllDevices(ctx); err != nil {
				remainingRetries--
				log.Error(err, "Failed to reconcile devices", "retries", map[string]any{"remaining": remainingRetries})

				if remainingRetries == 0 {
					log.Error(err, "Too many retries, stopping the controller")
					return err
				}
				continue
			} else {
				// Reset retries on successful cycle
				remainingRetries = 5
			}

		case <-ctx.Done():
			log.V(0).Info("Time-based reconciliation loop stopped due to context cancellation")
			return nil
		}
	}
}

func (c *RunCmd) webhookReconciliationLoop(ctx context.Context) error {
	log := ctrllog.FromContext(ctx).WithName("webhook")
	log.V(0).Info("Starting Tailscale webhook server")

	rt := chi.NewRouter()
	rt.Use(middleware.RealIP)
	middleware.DefaultLogger = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := log.WithValues(
				"request", map[string]any{"id": ulid.Make()},
				"client", map[string]any{"ip": r.RemoteAddr, "user-agent": r.UserAgent()},
			)

			log.V(2).Info("Webhook request received")
			next.ServeHTTP(w, r.WithContext(ctrllog.IntoContext(r.Context(), log)))
		})
	}
	rt.Use(middleware.Logger)
	rt.Use(middleware.Recoverer)

	rt.Post("/webhook", func(w http.ResponseWriter, r *http.Request) {
		log := ctrllog.FromContext(r.Context())
		log.V(1).Info("Processing Tailscale webhook request")

		var events []tsutils.WebhookEvent
		err := tsutils.VerifyWebhookSignature(ctx, r, c.Tailscale.Webhook.Secret, &events)
		if err != nil {
			log.Error(err, "Failed to verify webhook signature", "response.status", "UNAUTHORIZED")
			http.Error(w, "401 Invalid request signature", http.StatusUnauthorized)
			return
		}

		log.V(2).Info("Webhook signature verified successfully", "events", map[string]any{"count": len(events)})

		var errs *multierror.Error
		for i, event := range events {
			log := log.WithValues(
				"index", i,
				"event", event,
			)

			if event.Type != string(tailscale.WebhookNodeCreated) && event.Type != string(tailscale.WebhookNodeDeleted) {
				log.V(1).Info(fmt.Sprintf("Skipping event with unsupported type '%s', expecting '%s' or '%s'", event.Type, tailscale.WebhookNodeCreated, tailscale.WebhookNodeDeleted))
				continue
			}

			log.V(1).Info("Processing device event")
			_, err = c.reconciler.Reconcile(ctrllog.IntoContext(ctx, log), reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      event.Data.DeviceName,
					Namespace: c.Namespace,
				},
			})

			if err != nil {
				log.Error(err, "Failed to reconcile device from webhook event")
				errs = multierror.Append(errs, err)
			} else {
				log.V(1).Info("Successfully reconciled device from webhook event")
			}
		}

		if err := errs.ErrorOrNil(); err != nil {
			log.Error(err, "Webhook processing completed with error",
				"events", map[string]any{
					"total":     len(events),
					"processed": len(events) - errs.Len(),
					"failed":    errs.Len(),
				},
			)
			http.Error(w, "500 Failed to reconcile devices", http.StatusInternalServerError)
			return
		}
		log.V(0).Info("Webhook processing completed",
			"events", map[string]any{
				"total":     len(events),
				"processed": len(events),
				"failed":    0,
			},
		)
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.Tailscale.Webhook.Port),
		Handler: rt,
	}

	log.V(0).Info("Webhook server starting", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error(err, "Webhook server stopped with error")
		return err
	}

	log.V(0).Info("Webhook server successfully stopped")
	return nil
}
