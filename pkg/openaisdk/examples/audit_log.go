package examples

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
	"github.com/sirupsen/logrus"
)

var auditLogLog = logrus.WithFields(logrus.Fields{
	"component": "audit-log-example",
})

// formatAuditLog returns a clean string representation of an audit log
func formatAuditLog(log *openaisdk.AuditLog) map[string]interface{} {
	effectiveAt := time.Unix(log.EffectiveAt, 0).Format(time.RFC3339)

	fields := map[string]interface{}{
		"id":           log.ID,
		"type":         log.Type,
		"effective_at": effectiveAt,
	}

	if log.Actor != nil {
		fields["actor"] = map[string]interface{}{
			"type": log.Actor.Type,
		}
		if log.Actor.APIKey != nil {
			fields["actor"].(map[string]interface{})["api_key"] = map[string]interface{}{
				"id":   log.Actor.APIKey.ID,
				"type": log.Actor.APIKey.Type,
			}
			if log.Actor.APIKey.User != nil {
				fields["actor"].(map[string]interface{})["api_key"].(map[string]interface{})["user"] = map[string]interface{}{
					"id":    log.Actor.APIKey.User.ID,
					"email": log.Actor.APIKey.User.Email,
				}
			}
			if log.Actor.APIKey.ServiceAccount != nil {
				fields["actor"].(map[string]interface{})["api_key"].(map[string]interface{})["service_account"] = map[string]interface{}{
					"id": log.Actor.APIKey.ServiceAccount.ID,
				}
			}
		}
		if log.Actor.Session != nil {
			fields["actor"].(map[string]interface{})["session"] = map[string]interface{}{
				"ip_address": log.Actor.Session.IpAddress,
			}
			if log.Actor.Session.User != nil {
				fields["actor"].(map[string]interface{})["session"].(map[string]interface{})["user"] = map[string]interface{}{
					"id":    log.Actor.Session.User.ID,
					"email": log.Actor.Session.User.Email,
				}
			}
		}
	}

	if log.ApiKeyCreated != nil {
		fields["api_key.created"] = map[string]interface{}{
			"id":   log.ApiKeyCreated.ID,
			"data": log.ApiKeyCreated.Data,
		}
		if log.ApiKeyCreated.Data != nil {
			fields["api_key.created"].(map[string]interface{})["data"] = map[string]interface{}{
				"scopes": log.ApiKeyCreated.Data.Scopes,
			}
		}
	}

	if log.ApiKeyDeleted != nil {
		fields["api_key.deleted"] = map[string]interface{}{
			"id": log.ApiKeyDeleted.ID,
		}
	}

	if log.ApiKeyUpdated != nil {
		fields["api_key.updated"] = map[string]interface{}{
			"id": log.ApiKeyUpdated.ID,
		}
		if log.ApiKeyUpdated.ChangeRequested != nil {
			fields["api_key.updated"].(map[string]interface{})["change_requested"] = map[string]interface{}{
				"scopes": log.ApiKeyUpdated.ChangeRequested.Scopes,
			}
		}
	}

	if log.CertificateCreated != nil {
		fields["certificate.created"] = map[string]interface{}{
			"id":   log.CertificateCreated.ID,
			"name": log.CertificateCreated.Name,
		}
	}

	if log.CertificateDeleted != nil {
		fields["certificate.deleted"] = map[string]interface{}{
			"certificate": log.CertificateDeleted.Certificate,
			"id":          log.CertificateDeleted.ID,
			"name":        log.CertificateDeleted.Name,
		}
	}

	if log.CertificateUpdated != nil {
		fields["certificate.updated"] = map[string]interface{}{
			"id":   log.CertificateUpdated.ID,
			"name": log.CertificateUpdated.Name,
		}
	}

	if log.CertificatesActivated != nil && log.CertificatesActivated.Certificates != nil {
		fields["certificates.activated"] = map[string]interface{}{
			"certificates": log.CertificatesActivated.Certificates,
		}
	}

	if log.CertificatesDeactivated != nil {
		fields["certificates.deactivated"] = map[string]interface{}{
			"certificates": log.CertificatesDeactivated.Certificates,
		}
	}

	if log.CheckpointPermissionCreated != nil {
		fields["checkpoint_permission.created"] = map[string]interface{}{
			"id": log.CheckpointPermissionCreated.ID,
		}
		if log.CheckpointPermissionCreated.Data != nil {
			fields["checkpoint_permission.created"].(map[string]interface{})["data"] = map[string]interface{}{
				"fine_tuned_model_checkpoint": log.CheckpointPermissionCreated.Data.FineTunedModelCheckpoint,
				"project_id":                  log.CheckpointPermissionCreated.Data.ProjectID,
			}
		}
	}

	if log.CheckpointPermissionDeleted != nil {
		fields["checkpoint_permission.deleted"] = map[string]interface{}{
			"id": log.CheckpointPermissionDeleted.ID,
		}
	}

	if log.InviteAccepted != nil {
		fields["invite.accepted"] = map[string]interface{}{
			"id": log.InviteAccepted.ID,
		}
	}

	if log.InviteDeleted != nil {
		fields["invite.deleted"] = map[string]interface{}{
			"id": log.InviteDeleted.ID,
		}
	}

	if log.InviteSent != nil {
		fields["invite.sent"] = map[string]interface{}{
			"id": log.InviteSent.ID,
		}
		if log.InviteSent.Data != nil {
			fields["invite.sent"].(map[string]interface{})["data"] = map[string]interface{}{
				"email": log.InviteSent.Data.Email,
				"role":  log.InviteSent.Data.Role,
			}
		}
	}

	if log.LoginFailed != nil {
		fields["login.failed"] = map[string]interface{}{
			"error_code":    log.LoginFailed.ErrorCode,
			"error_message": log.LoginFailed.ErrorMessage,
		}
	}

	if log.LogoutFailed != nil {
		fields["logout.failed"] = map[string]interface{}{
			"error_code":    log.LogoutFailed.ErrorCode,
			"error_message": log.LogoutFailed.ErrorMessage,
		}
	}

	if log.OrganizationUpdated != nil {
		fields["organization.updated"] = map[string]interface{}{
			"id": log.OrganizationUpdated.ID,
		}
		if log.OrganizationUpdated.ChangesRequested != nil {
			fields["organization.updated"].(map[string]interface{})["changes_requested"] = map[string]interface{}{
				"description": log.OrganizationUpdated.ChangesRequested.Description,
				"name":        log.OrganizationUpdated.ChangesRequested.Name,
				"title":       log.OrganizationUpdated.ChangesRequested.Title,
			}
			if log.OrganizationUpdated.ChangesRequested.Settings != nil {
				fields["organization.updated"].(map[string]interface{})["changes_requested"].(map[string]interface{})["settings"] = map[string]interface{}{
					"threads_ui_visibility":      log.OrganizationUpdated.ChangesRequested.Settings.ThreadsUiVisibility,
					"usage_dashboard_visibility": log.OrganizationUpdated.ChangesRequested.Settings.UsageDashboardVisibility,
				}
			}
		}
	}

	if log.Project != nil {
		fields["project"] = map[string]interface{}{
			"id":   log.Project.ID,
			"name": log.Project.Name,
		}
	}

	if log.ProjectArchived != nil {
		fields["project.archived"] = map[string]interface{}{
			"id": log.ProjectArchived.ID,
		}
	}

	if log.ProjectCreated != nil {
		fields["project.created"] = map[string]interface{}{
			"id": log.ProjectCreated.ID,
		}
		if log.ProjectCreated.Data != nil {
			fields["project.created"].(map[string]interface{})["data"] = map[string]interface{}{
				"name":  log.ProjectCreated.Data.Name,
				"title": log.ProjectCreated.Data.Title,
			}
		}
	}

	if log.ProjectUpdated != nil {
		fields["project.updated"] = map[string]interface{}{
			"id": log.ProjectUpdated.ID,
		}
		if log.ProjectUpdated.ChangesRequested != nil {
			fields["project.updated"].(map[string]interface{})["changes_requested"] = map[string]interface{}{
				"title": log.ProjectUpdated.ChangesRequested.Title,
			}
		}
	}

	if log.RateLimitDeleted != nil {
		fields["rate_limit.deleted"] = map[string]interface{}{
			"id": log.RateLimitDeleted.ID,
		}
	}

	if log.RateLimitUpdated != nil {
		fields["rate_limit.updated"] = map[string]interface{}{
			"id": log.RateLimitUpdated.ID,
		}
		if log.RateLimitUpdated.ChangesRequested != nil {
			fields["rate_limit.updated"].(map[string]interface{})["changes_requested"] = map[string]interface{}{
				"batch_1_day_max_input_tokens":     log.RateLimitUpdated.ChangesRequested.Batch1DayMaxInputTokens,
				"max_audio_megabytes_per_1_minute": log.RateLimitUpdated.ChangesRequested.MaxAudioMegabytesPer1Minute,
				"max_images_per_1_minute":          log.RateLimitUpdated.ChangesRequested.MaxImagesPer1Minute,
				"max_requests_per_1_day":           log.RateLimitUpdated.ChangesRequested.MaxRequestsPer1Day,
				"max_requests_per_1_minute":        log.RateLimitUpdated.ChangesRequested.MaxRequestsPer1Minute,
				"max_tokens_per_1_minute":          log.RateLimitUpdated.ChangesRequested.MaxTokensPer1Minute,
			}
		}
	}

	if log.ServiceAccountCreated != nil {
		fields["service_account.created"] = map[string]interface{}{
			"id": log.ServiceAccountCreated.ID,
		}
		if log.ServiceAccountCreated.Data != nil {
			fields["service_account.created"].(map[string]interface{})["data"] = map[string]interface{}{
				"role": log.ServiceAccountCreated.Data.Role,
			}
		}
	}

	if log.ServiceAccountDeleted != nil {
		fields["service_account.deleted"] = map[string]interface{}{
			"id": log.ServiceAccountDeleted.ID,
		}
	}

	if log.ServiceAccountUpdated != nil {
		fields["service_account.updated"] = map[string]interface{}{
			"id": log.ServiceAccountUpdated.ID,
		}
		if log.ServiceAccountUpdated.ChangesRequested != nil {
			fields["service_account.updated"].(map[string]interface{})["changes_requested"] = map[string]interface{}{
				"role": log.ServiceAccountUpdated.ChangesRequested.Role,
			}
		}
	}

	if log.UserAdded != nil {
		fields["user.added"] = map[string]interface{}{
			"id": log.UserAdded.ID,
		}
		if log.UserAdded.Data != nil {
			fields["user.added"].(map[string]interface{})["data"] = map[string]interface{}{
				"role": log.UserAdded.Data.Role,
			}
		}
	}

	if log.UserDeleted != nil {
		fields["user.deleted"] = map[string]interface{}{
			"id": log.UserDeleted.ID,
		}
	}

	if log.UserUpdated != nil {
		fields["user.updated"] = map[string]interface{}{
			"id": log.UserUpdated.ID,
		}
		if log.UserUpdated.ChangesRequested != nil {
			fields["user.updated"].(map[string]interface{})["changes_requested"] = map[string]interface{}{
				"role": log.UserUpdated.ChangesRequested.Role,
			}
		}
	}

	return fields
}

// formatAuditLogList returns a clean string representation of audit log list
func formatAuditLogList(list *openaisdk.AuditLogListResponse) string {
	var logs []map[string]interface{}

	for _, log := range list.Data {

		logData := formatAuditLog(&log)

		logs = append(logs, logData)
	}

	fields := map[string]interface{}{
		"count":    len(list.Data),
		"logs":     logs,
		"has_more": list.HasMore,
	}

	b, _ := json.Marshal(fields)
	return string(b)
}

// AuditLogExample demonstrates how to use the OpenAI Audit Logs API
func AuditLogExample() {
	// Create a new client with your API key
	client := openaisdk.NewClient(option.WithAPIKey("api-admin-key"))

	// Set up the context
	ctx := context.Background()

	// List all audit logs

	auditLogs, err := client.AuditLogs.List(ctx)
	if err != nil {
		auditLogLog.WithError(err).Error("Failed to list audit logs")
		return
	}

	auditLogLog.Infof("Retrieved %d audit logs: %s", len(auditLogs.Data), formatAuditLogList(auditLogs))

}
