package notificationconfiguration

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	miniov1beta1 "github.com/rossigee/provider-minio/apis/minio/v1beta1"
)

func (nc *notificationClient) emitCreationEvent(cr *miniov1beta1.NotificationConfiguration) {
	nc.recorder.Event(cr, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Created",
		Message: "Webhook notification configuration successfully created",
	})
}

func (nc *notificationClient) emitDeletionEvent(cr *miniov1beta1.NotificationConfiguration) {
	nc.recorder.Event(cr, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Deleted",
		Message: "Webhook notification configuration successfully deleted",
	})
}

func (nc *notificationClient) emitUpdateEvent(cr *miniov1beta1.NotificationConfiguration) {
	nc.recorder.Event(cr, event.Event{
		Type:    event.TypeNormal,
		Reason:  "Updated",
		Message: "Webhook notification configuration successfully updated",
	})
}
