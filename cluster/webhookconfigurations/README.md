# Webhook Configurations

The manifests in this directory are applied by the `crossplane-init` init
container. See `internal/initializer/webhook_configurations.go`.

The initializer also removes old webhook configurations that aren't used
anymore. This is necessary to avoid requests failing when we remove a webhook
endpoint that we no longer need.

Currently `crossplane-init` will only remove webhook entries from the
ValidatingWebhookConfiguration named `validating-webhook-configuration`. It
won't delete the configuration. If the configuration is empty you can delete it
manually.