package openaisdk

import (
	"context"
	"net/http"

	"github.com/sgl-project/sgl-ome/pkg/openaisdk/apijson"
	"github.com/sgl-project/sgl-ome/pkg/openaisdk/option"
)

// AuditLogService contains methods for interacting with the OpenAI Audit Logs API.
// For more details see https://platform.openai.com/docs/api-reference/audit-logs
type AuditLogService struct {
	Options []option.RequestOption
}

// NewAuditLogService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewAuditLogService(opts ...option.RequestOption) (r *AuditLogService) {
	r = &AuditLogService{}
	r.Options = opts
	return
}

// List returns a list of audit logs based on the provided filters.
// For more details see https://platform.openai.com/docs/api-reference/audit-logs/list
func (r *AuditLogService) List(ctx context.Context, opts ...option.RequestOption) (res *AuditLogListResponse, err error) {
	opts = append(r.Options[:], opts...)
	err = option.ExecuteNewRequest(ctx, http.MethodGet, "organization/audit_logs", nil, &res, opts...)
	return
}

// AuditLogListParams defines the parameters for listing audit logs
type AuditLogListParams struct {
	// Return only events performed by users with these emails
	ActorEmails []string `json:"actor_emails,omitempty"`
	// Return only events performed by these actors. Can be a user ID, a service account ID, or an api key tracking ID
	ActorIDs []string `json:"actor_ids,omitempty"`
	// A cursor for use in pagination. after is an object ID that defines your place in the list
	After *string `json:"after,omitempty"`
	// A cursor for use in pagination. before is an object ID that defines your place in the list
	Before *string `json:"before,omitempty"`
	// Return only events whose effective_at (Unix seconds) is in this range
	EffectiveAt *EffectiveAtParams `json:"effective_at,omitempty"`
	// Return only events with a type in one of these values
	EventTypes []string `json:"event_types,omitempty"`
	// A limit on the number of objects to be returned. Limit can range between 1 and 100, and the default is 20
	Limit *int `json:"limit,omitempty"`
	// Return only events for these projects
	ProjectIDs []string `json:"project_ids,omitempty"`
	// Return only events performed on these targets. For example, a project ID updated
	ResourceIDs []string `json:"resource_ids,omitempty"`
}

// EffectiveAtParams defines the range parameters for filtering by effective_at timestamp
type EffectiveAtParams struct {
	// Return only events whose effective_at (Unix seconds) is greater than this value
	GT *int64 `json:"gt,omitempty"`
	// Return only events whose effective_at (Unix seconds) is greater than or equal to this value
	GTE *int64 `json:"gte,omitempty"`
	// Return only events whose effective_at (Unix seconds) is less than this value
	LT *int64 `json:"lt,omitempty"`
	// Return only events whose effective_at (Unix seconds) is less than or equal to this value
	LTE *int64 `json:"lte,omitempty"`
}

// AuditLogListResponse is the response struct for listing audit logs
type AuditLogListResponse struct {
	// The object type, which is always "list"
	Object string `json:"object"`
	// Array of audit log objects
	Data []AuditLog `json:"data"`
	// ID of the first audit log in the returned list
	FirstID string `json:"first_id"`
	// ID of the last audit log in the returned list
	LastID string `json:"last_id"`
	// Whether there are more audit logs available
	HasMore bool `json:"has_more"`
	// JSON metadata
	JSON auditLogListJSON `json:"-"`
}

// auditLogListJSON contains the JSON metadata for the struct [AuditLogListResponse]
type auditLogListJSON struct {
	Object      apijson.Field
	Data        apijson.Field
	FirstID     apijson.Field
	LastID      apijson.Field
	HasMore     apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

// UnmarshalJSON implements the json.Unmarshaler interface for AuditLogListResponse
func (r *AuditLogListResponse) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// Actor represents the entity that performed an action
type Actor struct {
	APIKey  *ActorApiKey `json:"api_key"`
	Session *Session     `json:"session"`
	Type    string       `json:"type"`
	JSON    actorJSON    `json:"-"`
}

// actorJSON contains the JSON metadata for the struct [Actor]
type actorJSON struct {
	Type        apijson.Field
	ID          apijson.Field
	Name        apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

// UnmarshalJSON implements the json.Unmarshaler interface for Actor
func (r *Actor) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type User struct {
	Email string   `json:"email"`
	ID    string   `json:"id"`
	JSON  userJSON `json:"-"`
}

type userJSON struct {
	Email       apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *User) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ServiceAccount struct {
	ID   string             `json:"id"`
	JSON serviceAccountJSON `json:"-"`
}

type serviceAccountJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ServiceAccount) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type Session struct {
	IpAddress string      `json:"ip_address"`
	User      *User       `json:"user"`
	JSON      sessionJSON `json:"-"`
}

type sessionJSON struct {
	IpAddress   apijson.Field
	User        apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *Session) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ActorApiKey struct {
	ID             string          `json:"id"`
	ServiceAccount *ServiceAccount `json:"service_account"`
	Type           string          `json:"type"`
	User           *User           `json:"user"`
	JSON           actorApiKeyJSON `json:"-"`
}

type actorApiKeyJSON struct {
	ID             apijson.Field
	ServiceAccount apijson.Field
	Type           apijson.Field
	User           apijson.Field
	raw            string //nolint:unused // Used by apijson for deserialization
	ExtraFields    map[string]apijson.Field
}

func (r *ActorApiKey) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type Data struct {
	Scopes []string `json:"scopes"`
	JSON   dataJSON `json:"-"`
}

type dataJSON struct {
	Scopes      apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *Data) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ApiKeyCreated struct {
	Data *Data             `json:"data"`
	ID   string            `json:"id"`
	JSON apiKeyCreatedJSON `json:"-"`
}

type apiKeyCreatedJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ApiKeyCreated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ApiKeyDeleted struct {
	ID string `json:"id"`
}

type ApiKeyUpdated struct {
	ChangeRequested *Data  `json:"changes_requested"`
	ID              string `json:"id"`
}

type CertificateCreated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CertificateDeleted struct {
	Certificate string `json:"certificate"`
	ID          string `json:"id"`
	Name        string `json:"name"`
}

type CertificateUpdated struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Certificate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CertificatesActivated struct {
	Certificates []Certificate             `json:"certificates"`
	JSON         certificatesActivatedJSON `json:"-"`
}

type certificatesActivatedJSON struct {
	Certificates apijson.Field
	raw          string //nolint:unused // Used by apijson for deserialization
	ExtraFields  map[string]apijson.Field
}

func (r *CertificatesActivated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type CertificatesDeactivated struct {
	Certificates []Certificate               `json:"certificates"`
	JSON         certificatesDeactivatedJSON `json:"-"`
}

type certificatesDeactivatedJSON struct {
	Certificates apijson.Field
	raw          string //nolint:unused // Used by apijson for deserialization
	ExtraFields  map[string]apijson.Field
}

func (r *CertificatesDeactivated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type CheckpointPermissionCreated struct {
	Data *CheckpointPermissionCreatedData `json:"data"`
	ID   string                           `json:"id"`
	JSON checkpointPermissionCreatedJSON  `json:"-"`
}

type CheckpointPermissionCreatedData struct {
	FineTunedModelCheckpoint string `json:"fine_tuned_model_checkpoint"`
	ProjectID                string `json:"project_id"`
}

type checkpointPermissionCreatedJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *CheckpointPermissionCreated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type CheckpointPermissionDeleted struct {
	ID   string                          `json:"id"`
	JSON checkpointPermissionDeletedJSON `json:"-"`
}

type checkpointPermissionDeletedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *CheckpointPermissionDeleted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type InviteAccepted struct {
	ID   string             `json:"id"`
	JSON inviteAcceptedJSON `json:"-"`
}

type inviteAcceptedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *InviteAccepted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type InviteDeleted struct {
	ID   string            `json:"id"`
	JSON inviteDeletedJSON `json:"-"`
}

type inviteDeletedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *InviteDeleted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type InviteSent struct {
	Data *InviteSentData `json:"data"`
	ID   string          `json:"id"`
	JSON inviteSentJSON  `json:"-"`
}

type InviteSentData struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type inviteSentJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *InviteSent) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type LoginFailed struct {
	ErrorCode    string          `json:"error_code"`
	ErrorMessage string          `json:"error_message"`
	JSON         loginFailedJSON `json:"-"`
}

type loginFailedJSON struct {
	ErrorCode    apijson.Field
	ErrorMessage apijson.Field
	raw          string //nolint:unused // Used by apijson for deserialization
	ExtraFields  map[string]apijson.Field
}

func (r *LoginFailed) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type LogoutFailed struct {
	ErrorCode    string           `json:"error_code"`
	ErrorMessage string           `json:"error_message"`
	JSON         logoutFailedJSON `json:"-"`
}

type logoutFailedJSON struct {
	ErrorCode    apijson.Field
	ErrorMessage apijson.Field
	raw          string //nolint:unused // Used by apijson for deserialization
	ExtraFields  map[string]apijson.Field
}

func (r *LogoutFailed) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type OrganizationUpdated struct {
	ChangesRequested *OrganizationUpdatedChanges `json:"changes_requested"`
	ID               string                      `json:"id"`
	JSON             organizationUpdatedJSON     `json:"-"`
}

type OrganizationUpdatedChanges struct {
	Description string                `json:"description"`
	Name        string                `json:"name"`
	Settings    *OrganizationSettings `json:"settings"`
	Title       string                `json:"title"`
}

type OrganizationSettings struct {
	ThreadsUiVisibility      string                   `json:"threads_ui_visibility"`
	UsageDashboardVisibility string                   `json:"usage_dashboard_visibility"`
	JSON                     organizationSettingsJSON `json:"-"`
}

type organizationSettingsJSON struct {
	ThreadsUiVisibility      apijson.Field
	UsageDashboardVisibility apijson.Field
	raw                      string //nolint:unused // Used by apijson for deserialization
	ExtraFields              map[string]apijson.Field
}

func (r *OrganizationSettings) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

// organizationUpdatedJSON contains the JSON metadata for the struct [OrganizationUpdated]
type organizationUpdatedJSON struct {
	ChangesRequested apijson.Field
	ID               apijson.Field
	raw              string //nolint:unused // Used by apijson for deserialization
	ExtraFields      map[string]apijson.Field
}

func (r *OrganizationUpdated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type auditLogProject struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	JSON auditLogProjectJSON `json:"-"`
}

type auditLogProjectJSON struct {
	ID          apijson.Field
	Name        apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *auditLogProject) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ProjectArchived struct {
	ID   string              `json:"id"`
	JSON projectArchivedJSON `json:"-"`
}

type projectArchivedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ProjectArchived) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ProjectCreated struct {
	Data *ProjectCreatedData `json:"data"`
	ID   string              `json:"id"`
	JSON projectCreatedJSON  `json:"-"`
}

type ProjectCreatedData struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type projectCreatedJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ProjectCreated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ProjectUpdated struct {
	ChangesRequested *ProjectUpdatedChanges `json:"changes_requested"`
	ID               string                 `json:"id"`
	JSON             projectUpdatedJSON     `json:"-"`
}

type ProjectUpdatedChanges struct {
	Title string `json:"title"`
}

type projectUpdatedJSON struct {
	ChangesRequested apijson.Field
	ID               apijson.Field
	raw              string //nolint:unused // Used by apijson for deserialization
	ExtraFields      map[string]apijson.Field
}

func (r *ProjectUpdated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type RateLimitDeleted struct {
	ID   string               `json:"id"`
	JSON rateLimitDeletedJSON `json:"-"`
}

type rateLimitDeletedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *RateLimitDeleted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type RateLimitUpdated struct {
	ChangesRequested *RateLimitUpdatedChanges `json:"changes_requested"`
	ID               string                   `json:"id"`
	JSON             rateLimitUpdatedJSON     `json:"-"`
}

type RateLimitUpdatedChanges struct {
	Batch1DayMaxInputTokens     *int `json:"batch_1_day_max_input_tokens,omitempty"`
	MaxAudioMegabytesPer1Minute *int `json:"max_audio_megabytes_per_1_minute,omitempty"`
	MaxImagesPer1Minute         *int `json:"max_images_per_1_minute,omitempty"`
	MaxRequestsPer1Day          *int `json:"max_requests_per_1_day,omitempty"`
	MaxRequestsPer1Minute       *int `json:"max_requests_per_1_minute,omitempty"`
	MaxTokensPer1Minute         *int `json:"max_tokens_per_1_minute,omitempty"`
}

type rateLimitUpdatedJSON struct {
	ChangesRequested apijson.Field
	ID               apijson.Field
	raw              string //nolint:unused // Used by apijson for deserialization
	ExtraFields      map[string]apijson.Field
}

func (r *RateLimitUpdated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ServiceAccountCreated struct {
	Data *ServiceAccountCreatedData `json:"data"`
	ID   string                     `json:"id"`
	JSON serviceAccountCreatedJSON  `json:"-"`
}

type ServiceAccountCreatedData struct {
	Role string `json:"role"`
}

type serviceAccountCreatedJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ServiceAccountCreated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ServiceAccountDeleted struct {
	ID   string                    `json:"id"`
	JSON serviceAccountDeletedJSON `json:"-"`
}

type serviceAccountDeletedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *ServiceAccountDeleted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type ServiceAccountUpdated struct {
	ChangesRequested *ServiceAccountUpdatedChanges `json:"changes_requested"`
	ID               string                        `json:"id"`
	JSON             serviceAccountUpdatedJSON     `json:"-"`
}

type ServiceAccountUpdatedChanges struct {
	Role string `json:"role"`
}

type serviceAccountUpdatedJSON struct {
	ChangesRequested apijson.Field
	ID               apijson.Field
	raw              string //nolint:unused // Used by apijson for deserialization
	ExtraFields      map[string]apijson.Field
}

func (r *ServiceAccountUpdated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type UserAdded struct {
	Data *UserAddedData `json:"data"`
	ID   string         `json:"id"`
	JSON userAddedJSON  `json:"-"`
}

type UserAddedData struct {
	Role string `json:"role"`
}

type userAddedJSON struct {
	Data        apijson.Field
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *UserAdded) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type UserDeleted struct {
	ID   string          `json:"id"`
	JSON userDeletedJSON `json:"-"`
}

type userDeletedJSON struct {
	ID          apijson.Field
	raw         string //nolint:unused // Used by apijson for deserialization
	ExtraFields map[string]apijson.Field
}

func (r *UserDeleted) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type UserUpdated struct {
	ChangesRequested *UserUpdatedChanges `json:"changes_requested"`
	ID               string              `json:"id"`
	JSON             userUpdatedJSON     `json:"-"`
}

type UserUpdatedChanges struct {
	Role string `json:"role"`
}

type userUpdatedJSON struct {
	ChangesRequested apijson.Field
	ID               apijson.Field
	raw              string //nolint:unused // Used by apijson for deserialization
	ExtraFields      map[string]apijson.Field
}

func (r *UserUpdated) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}

type AuditLog struct {
	Actor                       *Actor                       `json:"actor"`
	ApiKeyCreated               *ApiKeyCreated               `json:"api_key.created"`
	ApiKeyDeleted               *ApiKeyDeleted               `json:"api_key.deleted"`
	ApiKeyUpdated               *ApiKeyUpdated               `json:"api_key.updated"`
	CertificateCreated          *CertificateCreated          `json:"certificate.created"`
	CertificateDeleted          *CertificateDeleted          `json:"certificate.deleted"`
	CertificateUpdated          *CertificateUpdated          `json:"certificate.updated"`
	CertificatesActivated       *CertificatesActivated       `json:"certificate.activated"`
	CertificatesDeactivated     *CertificatesDeactivated     `json:"certificate.deactivated"`
	CheckpointPermissionCreated *CheckpointPermissionCreated `json:"checkpoint_permission.created"`
	CheckpointPermissionDeleted *CheckpointPermissionDeleted `json:"checkpoint_permission.deleted"`
	EffectiveAt                 int64                        `json:"effective_at"`
	ID                          string                       `json:"id"`
	InviteAccepted              *InviteAccepted              `json:"invite.accepted"`
	InviteDeleted               *InviteDeleted               `json:"invite.deleted"`
	InviteSent                  *InviteSent                  `json:"invite.sent"`
	LoginFailed                 *LoginFailed                 `json:"login.failed"`
	LogoutFailed                *LogoutFailed                `json:"logout.failed"`
	OrganizationUpdated         *OrganizationUpdated         `json:"organization.updated"`
	Project                     *auditLogProject             `json:"project"`
	ProjectArchived             *ProjectArchived             `json:"project.archived"`
	ProjectCreated              *ProjectCreated              `json:"project.created"`
	ProjectUpdated              *ProjectUpdated              `json:"project.updated"`
	RateLimitDeleted            *RateLimitDeleted            `json:"rate_limit.deleted"`
	RateLimitUpdated            *RateLimitUpdated            `json:"rate_limit.updated"`
	ServiceAccountCreated       *ServiceAccountCreated       `json:"service_account.created"`
	ServiceAccountDeleted       *ServiceAccountDeleted       `json:"service_account.deleted"`
	ServiceAccountUpdated       *ServiceAccountUpdated       `json:"service_account.updated"`
	Type                        string                       `json:"type"`
	UserAdded                   *UserAdded                   `json:"user.added"`
	UserDeleted                 *UserDeleted                 `json:"user.deleted"`
	UserUpdated                 *UserUpdated                 `json:"user.updated"`
	JSON                        auditLogJSON                 `json:"-"`
}

type auditLogJSON struct {
	Actor                       apijson.Field
	ApiKeyCreated               apijson.Field
	ApiKeyDeleted               apijson.Field
	ApiKeyUpdated               apijson.Field
	CertificateCreated          apijson.Field
	CertificateDeleted          apijson.Field
	CertificateUpdated          apijson.Field
	CertificatesActivated       apijson.Field
	CertificatesDeactivated     apijson.Field
	CheckpointPermissionCreated apijson.Field
	CheckpointPermissionDeleted apijson.Field
	EffectiveAt                 apijson.Field
	ID                          apijson.Field
	InviteAccepted              apijson.Field
	InviteDeleted               apijson.Field
	InviteSent                  apijson.Field
	LoginFailed                 apijson.Field
	LogoutFailed                apijson.Field
	OrganizationUpdated         apijson.Field
	Project                     apijson.Field
	ProjectArchived             apijson.Field
	ProjectCreated              apijson.Field
	ProjectUpdated              apijson.Field
	RateLimitDeleted            apijson.Field
	RateLimitUpdated            apijson.Field
	ServiceAccountCreated       apijson.Field
	ServiceAccountDeleted       apijson.Field
	ServiceAccountUpdated       apijson.Field
	Type                        apijson.Field
	UserAdded                   apijson.Field
	UserDeleted                 apijson.Field
	UserUpdated                 apijson.Field
	raw                         string //nolint:unused // Used by apijson for deserialization
	ExtraFields                 map[string]apijson.Field
}

func (r *AuditLog) UnmarshalJSON(data []byte) (err error) {
	return apijson.UnmarshalRoot(data, r)
}
