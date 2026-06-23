package notificationconfiguration

import "context"

// Disconnect is a noop for NotificationConfiguration.
func (nc *notificationClient) Disconnect(context.Context) error {
	return nil
}
