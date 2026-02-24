package whatsnyanmodel

import "service-platform/internal/config"

// WhatsmeowAppStateMutationMacs represents the whatsmeow_app_state_mutation_macs table
type WhatsmeowAppStateMutationMacs struct {
	JID      string `gorm:"column:jid;primaryKey"`
	Name     string `gorm:"column:name;primaryKey"`
	Version  int64  `gorm:"column:version;primaryKey"`
	IndexMAC []byte `gorm:"column:index_mac;primaryKey"`
	ValueMAC []byte `gorm:"column:value_mac"`
}

// TableName returns the database table name for WhatsmeowAppStateMutationMacs.
func (WhatsmeowAppStateMutationMacs) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBAppStateMutationMacs
}

// WhatsmeowAppStateSyncKeys represents the whatsmeow_app_state_sync_keys table
type WhatsmeowAppStateSyncKeys struct {
	JID         string `gorm:"column:jid;primaryKey"`
	KeyID       []byte `gorm:"column:key_id;primaryKey"`
	KeyData     []byte `gorm:"column:key_data"`
	Timestamp   int64  `gorm:"column:timestamp"`
	Fingerprint []byte `gorm:"column:fingerprint"`
}

// TableName returns the database table name for WhatsmeowAppStateSyncKeys.
func (WhatsmeowAppStateSyncKeys) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBAppStateSyncKeys
}

// WhatsmeowAppStateVersion represents the whatsmeow_app_state_version table
type WhatsmeowAppStateVersion struct {
	JID     string `gorm:"column:jid;primaryKey"`
	Name    string `gorm:"column:name;primaryKey"`
	Version int64  `gorm:"column:version"`
	Hash    []byte `gorm:"column:hash"`
}

// TableName returns the database table name for WhatsmeowAppStateVersion.
func (WhatsmeowAppStateVersion) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBAppStateVersions
}

// WhatsmeowChatSettings represents the whatsmeow_chat_settings table
type WhatsmeowChatSettings struct {
	OurJID     string `gorm:"column:our_jid;primaryKey"`
	ChatJID    string `gorm:"column:chat_jid;primaryKey"`
	MutedUntil int64  `gorm:"column:muted_until;default:0"`
	Pinned     bool   `gorm:"column:pinned;default:false"`
	Archived   bool   `gorm:"column:archived;default:false"`
}

// TableName returns the database table name for WhatsmeowChatSettings.
func (WhatsmeowChatSettings) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBChatSettings
}

// WhatsmeowContacts represents the whatsmeow_contacts table
type WhatsmeowContacts struct {
	OurJID        string  `gorm:"column:our_jid;primaryKey"`
	TheirJID      string  `gorm:"column:their_jid;primaryKey"`
	FirstName     *string `gorm:"column:first_name"`
	FullName      *string `gorm:"column:full_name"`
	PushName      *string `gorm:"column:push_name"`
	BusinessName  *string `gorm:"column:business_name"`
	RedactedPhone *string `gorm:"column:redacted_phone"`
}

// TableName returns the database table name for WhatsmeowContacts.
func (WhatsmeowContacts) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBContacts
}

// WhatsmeowDevice represents the whatsmeow_device table
type WhatsmeowDevice struct {
	JID              string  `gorm:"column:jid;primaryKey"`
	LID              *string `gorm:"column:lid"`
	FacebookUUID     *string `gorm:"column:facebook_uuid"`
	RegistrationID   int64   `gorm:"column:registration_id"`
	NoiseKey         []byte  `gorm:"column:noise_key"`
	IdentityKey      []byte  `gorm:"column:identity_key"`
	SignedPreKey     []byte  `gorm:"column:signed_pre_key"`
	SignedPreKeyID   int32   `gorm:"column:signed_pre_key_id"`
	SignedPreKeySig  []byte  `gorm:"column:signed_pre_key_sig"`
	AdvKey           []byte  `gorm:"column:adv_key"`
	AdvDetails       []byte  `gorm:"column:adv_details"`
	AdvAccountSig    []byte  `gorm:"column:adv_account_sig"`
	AdvAccountSigKey []byte  `gorm:"column:adv_account_sig_key"`
	AdvDeviceSig     []byte  `gorm:"column:adv_device_sig"`
	Platform         string  `gorm:"column:platform;default:''"`
	BusinessName     string  `gorm:"column:business_name;default:''"`
	PushName         string  `gorm:"column:push_name;default:''"`
	LIDMigrationTS   int64   `gorm:"column:lid_migration_ts;default:0"`
}

// TableName returns the database table name for WhatsmeowDevice.
func (WhatsmeowDevice) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBDevice
}

// WhatsmeowEventBuffer represents the whatsmeow_event_buffer table
type WhatsmeowEventBuffer struct {
	OurJID          string `gorm:"column:our_jid;primaryKey"`
	CiphertextHash  []byte `gorm:"column:ciphertext_hash;primaryKey"`
	Plaintext       []byte `gorm:"column:plaintext"`
	ServerTimestamp int64  `gorm:"column:server_timestamp"`
	InsertTimestamp int64  `gorm:"column:insert_timestamp"`
}

// TableName returns the database table name for WhatsmeowEventBuffer.
func (WhatsmeowEventBuffer) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBEventBuffer
}

// WhatsmeowIdentityKeys represents the whatsmeow_identity_keys table
type WhatsmeowIdentityKeys struct {
	OurJID   string `gorm:"column:our_jid;primaryKey"`
	TheirID  string `gorm:"column:their_id;primaryKey"`
	Identity []byte `gorm:"column:identity"`
}

// TableName returns the database table name for WhatsmeowIdentityKeys.
func (WhatsmeowIdentityKeys) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBIdentityKeys
}

// WhatsmeowLIDMap represents the whatsmeow_lid_map table
type WhatsmeowLIDMap struct {
	LID string `gorm:"column:lid;primaryKey"`
	PN  string `gorm:"column:pn;unique"`
}

// TableName returns the database table name for WhatsmeowLIDMap.
func (WhatsmeowLIDMap) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBLIDMap
}

// WhatsmeowMessageSecrets represents the whatsmeow_message_secrets table
type WhatsmeowMessageSecrets struct {
	OurJID    string `gorm:"column:our_jid;primaryKey"`
	ChatJID   string `gorm:"column:chat_jid;primaryKey"`
	SenderJID string `gorm:"column:sender_jid;primaryKey"`
	MessageID string `gorm:"column:message_id;primaryKey"`
	Key       []byte `gorm:"column:key"`
}

// TableName returns the database table name for WhatsmeowMessageSecrets.
func (WhatsmeowMessageSecrets) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBMessageSecrets
}

// WhatsmeowPreKeys represents the whatsmeow_pre_keys table
type WhatsmeowPreKeys struct {
	JID      string `gorm:"column:jid;primaryKey"`
	KeyID    int32  `gorm:"column:key_id;primaryKey"`
	Key      []byte `gorm:"column:key"`
	Uploaded bool   `gorm:"column:uploaded"`
}

// TableName returns the database table name for WhatsmeowPreKeys.
func (WhatsmeowPreKeys) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBPreKeys
}

// WhatsmeowPrivacyTokens represents the whatsmeow_privacy_tokens table
type WhatsmeowPrivacyTokens struct {
	OurJID    string `gorm:"column:our_jid;primaryKey"`
	TheirJID  string `gorm:"column:their_jid;primaryKey"`
	Token     []byte `gorm:"column:token"`
	Timestamp int64  `gorm:"column:timestamp"`
}

// TableName returns the database table name for WhatsmeowPrivacyTokens.
func (WhatsmeowPrivacyTokens) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBPrivacyTokens
}

// WhatsmeowSenderKeys represents the whatsmeow_sender_keys table
type WhatsmeowSenderKeys struct {
	OurJID    string `gorm:"column:our_jid;primaryKey"`
	ChatID    string `gorm:"column:chat_id;primaryKey"`
	SenderID  string `gorm:"column:sender_id;primaryKey"`
	SenderKey []byte `gorm:"column:sender_key"`
}

// TableName returns the database table name for WhatsmeowSenderKeys.
func (WhatsmeowSenderKeys) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBSenderKeys
}

// WhatsmeowSessions represents the whatsmeow_sessions table
type WhatsmeowSessions struct {
	OurJID  string `gorm:"column:our_jid;primaryKey"`
	TheirID string `gorm:"column:their_id;primaryKey"`
	Session []byte `gorm:"column:session"`
}

// TableName returns the database table name for WhatsmeowSessions.
func (WhatsmeowSessions) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBSessions
}

// WhatsmeowVersion represents the whatsmeow_version table
type WhatsmeowVersion struct {
	Version *int32 `gorm:"column:version"`
	Compat  *int32 `gorm:"column:compat"`
}

// TableName returns the database table name for WhatsmeowVersion.
func (WhatsmeowVersion) TableName() string {
	return config.ServicePlatform.Get().Whatsnyan.Tables.TBVersion
}
