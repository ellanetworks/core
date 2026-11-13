-- index idx_audit_logs_timestamp
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC,id DESC);

-- index idx_cell_positions_cell
CREATE UNIQUE INDEX idx_cell_positions_cell ON cell_positions(rat,mcc,mnc,cell_identity);

-- index idx_deferred_location_requests_active_supi
CREATE UNIQUE INDEX idx_deferred_location_requests_active_supi ON deferred_location_requests(supi)WHERE status = 'ACTIVE';

-- index idx_deferred_location_requests_supi
CREATE INDEX idx_deferred_location_requests_supi ON deferred_location_requests(supi);

-- index idx_deferred_location_requests_terminated_at
CREATE INDEX idx_deferred_location_requests_terminated_at ON deferred_location_requests(terminated_at);

-- index idx_diameter_peers_identity
CREATE UNIQUE INDEX idx_diameter_peers_identity ON diameter_peers(diameterIdentity COLLATE NOCASE)WHERE diameterIdentity != '';

-- index idx_diameter_peers_smsc
CREATE INDEX idx_diameter_peers_smsc ON diameter_peers(smscId);

-- index idx_flow_reports_destination_ip
CREATE INDEX idx_flow_reports_destination_ip ON flow_reports(destination_ip);

-- index idx_flow_reports_end_time
CREATE INDEX idx_flow_reports_end_time ON flow_reports(end_time,start_time);

-- index idx_flow_reports_protocol
CREATE INDEX idx_flow_reports_protocol ON flow_reports(protocol);

-- index idx_flow_reports_source_ip
CREATE INDEX idx_flow_reports_source_ip ON flow_reports(source_ip);

-- index idx_flow_reports_subscriber_id
CREATE INDEX idx_flow_reports_subscriber_id ON flow_reports(subscriber_id);

-- index idx_framed_routes_dn
CREATE INDEX idx_framed_routes_dn ON subscriber_framed_routes(dataNetworkID);

-- index idx_framed_routes_imsi_dn
CREATE INDEX idx_framed_routes_imsi_dn ON subscriber_framed_routes(imsi,dataNetworkID);

-- index idx_ims_initial_filter_criteria_as
CREATE INDEX idx_ims_initial_filter_criteria_as ON ims_initial_filter_criteria(applicationServerID);

-- index idx_ims_public_service_identities_as
CREATE INDEX idx_ims_public_service_identities_as ON ims_public_service_identities(applicationServerID);

-- index idx_ims_service_profile_assignments_sp
CREATE INDEX idx_ims_service_profile_assignments_sp ON ims_service_profile_assignments(serviceProfileID);

-- index idx_leases_imsi
CREATE INDEX idx_leases_imsi ON ip_leases(imsi);

-- index idx_leases_node
CREATE INDEX idx_leases_node ON ip_leases(nodeID);

-- index idx_leases_pool
CREATE INDEX idx_leases_pool ON ip_leases(poolID);

-- index idx_leases_session
CREATE INDEX idx_leases_session ON ip_leases(sessionID);

-- index idx_network_logs_direction
CREATE INDEX idx_network_logs_direction ON network_logs(direction);

-- index idx_network_logs_local_address
CREATE INDEX idx_network_logs_local_address ON network_logs(local_address);

-- index idx_network_logs_message_type
CREATE INDEX idx_network_logs_message_type ON network_logs(message_type);

-- index idx_network_logs_protocol
CREATE INDEX idx_network_logs_protocol ON network_logs(protocol);

-- index idx_network_logs_radio_name
CREATE INDEX idx_network_logs_radio_name ON network_logs(radio_name);

-- index idx_network_logs_radio_ref
CREATE INDEX idx_network_logs_radio_ref ON network_logs(radio_ref);

-- index idx_network_logs_remote_address
CREATE INDEX idx_network_logs_remote_address ON network_logs(remote_address);

-- index idx_network_logs_timestamp
CREATE INDEX idx_network_logs_timestamp ON network_logs(timestamp);

-- index idx_roaming_partner_plmns_partner
CREATE INDEX idx_roaming_partner_plmns_partner ON roaming_partner_plmns(partnerID);

-- index idx_roaming_partners_sepp_fqdn
CREATE UNIQUE INDEX idx_roaming_partners_sepp_fqdn ON roaming_partners(seppFQDN)WHERE seppFQDN IS NOT NULL;

-- index idx_slice_data_networks_data_network
CREATE INDEX idx_slice_data_networks_data_network ON slice_data_networks(dataNetworkID);

-- index idx_sms_message_waiting_updated
CREATE INDEX idx_sms_message_waiting_updated ON sms_message_waiting(updatedAt);

-- index idx_sms_smsc_service_centres_smsc
CREATE INDEX idx_sms_smsc_service_centres_smsc ON sms_smsc_service_centres(smscId);

-- index idx_sms_smscs_name
CREATE UNIQUE INDEX idx_sms_smscs_name ON sms_smscs(name COLLATE NOCASE);

-- index idx_subscribers_msisdn
CREATE UNIQUE INDEX idx_subscribers_msisdn ON subscribers(msisdn)WHERE msisdn != '';

-- index idx_ue_registrations_node
CREATE INDEX idx_ue_registrations_node ON ue_registrations(nodeID);

-- index idx_ue_registrations_version
CREATE INDEX idx_ue_registrations_version ON ue_registrations(version);

-- index idx_visitors_partner
CREATE INDEX idx_visitors_partner ON visitors(partnerID);

-- table api_tokens
CREATE TABLE "api_tokens"(id TEXT PRIMARY KEY,token_id TEXT NOT NULL UNIQUE,name TEXT NOT NULL,token_hash TEXT NOT NULL,user_id TEXT NOT NULL,expires_at DATETIME,FOREIGN KEY(user_id)REFERENCES users(id)ON DELETE CASCADE,UNIQUE(name,user_id));

-- table audit_logs
CREATE TABLE "audit_logs"(id TEXT PRIMARY KEY,timestamp INTEGER NOT NULL,level TEXT NOT NULL,actor TEXT NOT NULL DEFAULT '',action TEXT NOT NULL,ip TEXT NOT NULL DEFAULT '',details TEXT NOT NULL DEFAULT '');

-- table bgp_import_prefixes
CREATE TABLE bgp_import_prefixes(id INTEGER PRIMARY KEY AUTOINCREMENT,peerID INTEGER NOT NULL REFERENCES bgp_peers(id)ON DELETE CASCADE,prefix TEXT NOT NULL,maxLength INTEGER NOT NULL);

-- table bgp_peers
CREATE TABLE bgp_peers(id INTEGER PRIMARY KEY AUTOINCREMENT,address TEXT NOT NULL UNIQUE,remoteAS INTEGER NOT NULL,holdTime INTEGER NOT NULL DEFAULT 90,password TEXT NOT NULL DEFAULT '',description TEXT NOT NULL DEFAULT '');

-- table bgp_settings
CREATE TABLE bgp_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton = TRUE),enabled BOOLEAN NOT NULL DEFAULT FALSE,localAS INTEGER NOT NULL DEFAULT 64512,routerID TEXT NOT NULL DEFAULT '',listenAddress TEXT NOT NULL DEFAULT ':179');

-- table cell_positions
CREATE TABLE cell_positions(id TEXT PRIMARY KEY,rat TEXT NOT NULL,mcc TEXT NOT NULL,mnc TEXT NOT NULL,cell_identity TEXT NOT NULL,gnb_id TEXT,latitude REAL NOT NULL,longitude REAL NOT NULL,altitude REAL,uncertainty_semi_major REAL,uncertainty_semi_minor REAL,orientation_major INTEGER,confidence INTEGER,source TEXT NOT NULL DEFAULT 'provisioned',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);

-- table cluster_join_hmac
CREATE TABLE cluster_join_hmac(id INTEGER PRIMARY KEY CHECK(id = 1),hmacKey BLOB NOT NULL);

-- table cluster_join_tokens
CREATE TABLE "cluster_join_tokens"(id TEXT PRIMARY KEY,claimsJSON TEXT NOT NULL,expiresAt INTEGER NOT NULL,consumedAt INTEGER NOT NULL DEFAULT 0,consumedBy TEXT NOT NULL DEFAULT '');

-- table cluster_members
CREATE TABLE "cluster_members"(nodeID TEXT PRIMARY KEY,apiAddress TEXT NOT NULL,binaryVersion TEXT NOT NULL DEFAULT '',drainState TEXT NOT NULL DEFAULT 'active' CHECK(drainState IN('active','draining','drained')),drainUpdatedAt INTEGER NOT NULL DEFAULT 0,amfPointer INTEGER NOT NULL DEFAULT 0,displayName TEXT NOT NULL DEFAULT '');

-- table cluster_node_certs
CREATE TABLE "cluster_node_certs"(nodeID TEXT PRIMARY KEY,fingerprint TEXT NOT NULL UNIQUE,certPEM TEXT NOT NULL,addedAt INTEGER NOT NULL);

-- table daily_usage
CREATE TABLE "daily_usage"(epochDay INTEGER NOT NULL,imsi TEXT NOT NULL CHECK(length(imsi)BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),mcc TEXT CHECK(mcc GLOB '[0-9][0-9][0-9]'),mnc TEXT CHECK(mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),bytesUplink INTEGER NOT NULL DEFAULT 0 CHECK(bytesUplink >= 0),bytesDownlink INTEGER NOT NULL DEFAULT 0 CHECK(bytesDownlink >= 0),PRIMARY KEY(epochDay,imsi),CHECK((mcc IS NULL)=(mnc IS NULL)),CHECK(mcc IS NULL OR substr(imsi,1,length(mcc)+ length(mnc))= mcc || mnc));

-- table data_networks
CREATE TABLE "data_networks"(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,ipPool TEXT NOT NULL,dns TEXT NOT NULL,mtu INTEGER NOT NULL,ipv6Pool TEXT NOT NULL DEFAULT '',ims BOOLEAN NOT NULL DEFAULT FALSE CHECK(ims IN(0,1)));

-- table deferred_location_requests
CREATE TABLE deferred_location_requests(id TEXT PRIMARY KEY,supi TEXT NOT NULL CHECK(supi != ''),ldr_type TEXT NOT NULL CHECK(ldr_type IN('UE_AVAILABLE','PERIODIC','ENTERING_INTO_AREA','LEAVING_FROM_AREA','BEING_INSIDE_AREA','MOTION')),method TEXT NOT NULL CHECK(method IN('cell_id','ecid','gnss')),mode TEXT CHECK(mode IN('network_based','ue_assisted','standalone')),measurement_quantities TEXT,reporting_interval_s INTEGER CHECK(reporting_interval_s BETWEEN 1 AND 8639999),reporting_amount INTEGER CHECK(reporting_amount BETWEEN 1 AND 8639999),reporting_infinite INTEGER NOT NULL DEFAULT 0 CHECK(reporting_infinite IN(0,1)),scheduled_loc_time INTEGER,status TEXT NOT NULL CHECK(status IN('ACTIVE','TERMINATED')),termination_cause TEXT CHECK(termination_cause IN('NORMAL_TERMINATION','TERMINATION_BY_UE','TERMINATION_BY_NETWORK')),termination_reason TEXT CHECK(termination_reason IN('ue_deregistered','ue_mobility','positioning_failed','node_restart','node_shutdown')),created_at INTEGER NOT NULL,terminated_at INTEGER,CHECK((ldr_type = 'PERIODIC')=(reporting_interval_s IS NOT NULL AND reporting_amount IS NOT NULL)),CHECK(reporting_infinite = 0 OR ldr_type = 'PERIODIC'),CHECK(reporting_infinite = 1 OR reporting_amount IS NULL OR reporting_amount * reporting_interval_s <= 8639999),CHECK((status = 'ACTIVE')=(termination_cause IS NULL)),CHECK((status = 'ACTIVE')=(terminated_at IS NULL)),CHECK(termination_reason IS NULL OR termination_cause IS NOT NULL));

-- table diameter_peers
CREATE TABLE "diameter_peers"(id TEXT PRIMARY KEY,role TEXT NOT NULL CHECK(role IN('smsc','roaming')),diameterIdentity TEXT NOT NULL,address TEXT NOT NULL,port INTEGER NOT NULL,smscId TEXT REFERENCES sms_smscs(id)ON DELETE CASCADE,CHECK((role = 'smsc')=(smscId IS NOT NULL)));

-- table flow_accounting_settings
CREATE TABLE flow_accounting_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,enabled BOOLEAN NOT NULL DEFAULT TRUE,CHECK(singleton));

-- table flow_reports
CREATE TABLE "flow_reports"(id INTEGER PRIMARY KEY AUTOINCREMENT,subscriber_id TEXT NOT NULL,source_ip TEXT NOT NULL,destination_ip TEXT NOT NULL,source_port INTEGER NOT NULL DEFAULT 0,destination_port INTEGER NOT NULL DEFAULT 0,protocol INTEGER NOT NULL,packets INTEGER NOT NULL,bytes INTEGER NOT NULL,start_time INTEGER NOT NULL,end_time INTEGER NOT NULL,direction TEXT NOT NULL,action INT NOT NULL DEFAULT 0);

-- table fsm_state
CREATE TABLE fsm_state(id INTEGER PRIMARY KEY CHECK(id = 1),lastApplied INTEGER NOT NULL DEFAULT 0,lastAppliedTerm INTEGER NOT NULL DEFAULT 0,configuration BLOB,configurationIndex INTEGER NOT NULL DEFAULT 0,durableWrites INTEGER NOT NULL DEFAULT 0);

-- table home_network_keys
CREATE TABLE "home_network_keys"(id TEXT PRIMARY KEY,key_identifier INTEGER NOT NULL CHECK(key_identifier >= 0 AND key_identifier <= 255),scheme TEXT NOT NULL CHECK(scheme IN('A','B')),private_key TEXT NOT NULL,UNIQUE(key_identifier,scheme));

-- table ims_application_servers
CREATE TABLE ims_application_servers(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE CHECK(name != ''),serverName TEXT NOT NULL UNIQUE CHECK(serverName != ''));

-- table ims_initial_filter_criteria
CREATE TABLE ims_initial_filter_criteria(serviceProfileID TEXT NOT NULL REFERENCES ims_service_profiles(id)ON DELETE CASCADE,position INTEGER NOT NULL CHECK(position >= 0),applicationServerID TEXT NOT NULL REFERENCES ims_application_servers(id),triggerPoint TEXT CHECK(triggerPoint IS NULL OR json_valid(triggerPoint)),profilePart TEXT CHECK(profilePart IS NULL OR profilePart IN('REGISTERED','UNREGISTERED')),defaultHandling TEXT CHECK(defaultHandling IS NULL OR defaultHandling IN('SESSION_CONTINUED','SESSION_TERMINATED')),serviceInfo TEXT,includeRegisterRequest BOOLEAN NOT NULL CHECK(includeRegisterRequest IN(0,1)),includeRegisterResponse BOOLEAN NOT NULL CHECK(includeRegisterResponse IN(0,1)),PRIMARY KEY(serviceProfileID,position));

-- table ims_peers
CREATE TABLE ims_peers(id TEXT PRIMARY KEY,name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK(name != ''),originHost TEXT CHECK(originHost != '' AND originHost = lower(originHost)),realm TEXT CHECK(realm != '' AND realm = lower(realm)),addresses TEXT NOT NULL CHECK(CASE WHEN json_valid(addresses)THEN json_type(addresses)= 'array' AND json_array_length(addresses)BETWEEN 1 AND 16 ELSE 0 END),CHECK(originHost IS NOT NULL OR realm IS NOT NULL));

-- table ims_public_service_identities
CREATE TABLE ims_public_service_identities(identity TEXT PRIMARY KEY CHECK(identity != ''),applicationServerID TEXT NOT NULL REFERENCES ims_application_servers(id)ON DELETE CASCADE);

-- table ims_registration_sets
CREATE TABLE ims_registration_sets(imsi TEXT NOT NULL REFERENCES ims_registrations(imsi)ON DELETE CASCADE,registrationSet TEXT NOT NULL CHECK(registrationSet IN('mmtel','mcptt')),state INTEGER NOT NULL CHECK(state IN(0,1,2)),authPending BOOLEAN NOT NULL CHECK(authPending IN(0,1)),defaultIdentity TEXT NOT NULL,profileDigest TEXT NOT NULL CHECK(state = 0 OR profileDigest != ''),emergency BOOLEAN NOT NULL CHECK(emergency IN(0,1)AND(emergency = 0 OR state = 2)),CHECK(state != 0 OR authPending = 1),PRIMARY KEY(imsi,registrationSet));

-- table ims_registrations
CREATE TABLE ims_registrations(imsi TEXT PRIMARY KEY REFERENCES subscribers(imsi)ON DELETE CASCADE,serverName TEXT NOT NULL CHECK(serverName != ''),originHost TEXT NOT NULL CHECK(originHost != ''),originRealm TEXT NOT NULL CHECK(originRealm != ''),privateIdentity TEXT NOT NULL);

-- table ims_service_profile_assignments
CREATE TABLE ims_service_profile_assignments(profileID TEXT NOT NULL REFERENCES profiles(id)ON DELETE CASCADE,registrationSet TEXT NOT NULL CHECK(registrationSet IN('mmtel','mcptt')),serviceProfileID TEXT NOT NULL REFERENCES ims_service_profiles(id),PRIMARY KEY(profileID,registrationSet));

-- table ims_service_profiles
CREATE TABLE ims_service_profiles(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE CHECK(name != ''));

-- table ims_settings
CREATE TABLE ims_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton),homeDomain TEXT NOT NULL DEFAULT '' CHECK(homeDomain = lower(homeDomain)),advertiseVoice BOOLEAN NOT NULL DEFAULT TRUE,restrictPeers BOOLEAN NOT NULL DEFAULT FALSE);

-- table ip_leases
CREATE TABLE "ip_leases"(id TEXT PRIMARY KEY,poolID TEXT NOT NULL REFERENCES data_networks(id)ON DELETE CASCADE,addressBin BLOB NOT NULL,imsi TEXT NOT NULL CHECK(length(imsi)BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),sessionID INTEGER,type TEXT NOT NULL DEFAULT 'dynamic',createdAt INTEGER NOT NULL,nodeID TEXT NOT NULL DEFAULT '',poolType TEXT NOT NULL DEFAULT 'ipv4',UNIQUE(poolID,addressBin));

-- table ip_sm_gw_registrations
CREATE TABLE ip_sm_gw_registrations(imsi TEXT PRIMARY KEY REFERENCES subscribers(imsi)ON DELETE CASCADE,number TEXT NOT NULL CHECK(number != ''),diameterHost TEXT NOT NULL CHECK(diameterHost != ''),diameterRealm TEXT NOT NULL CHECK(diameterRealm != ''));

-- table jwt_secret
CREATE TABLE jwt_secret(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton = TRUE),secret BLOB NOT NULL);

-- table local_switch_settings
CREATE TABLE local_switch_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,enabled BOOLEAN NOT NULL DEFAULT FALSE,CHECK(singleton));

-- table location_reports
CREATE TABLE location_reports(request_id TEXT NOT NULL REFERENCES deferred_location_requests(id)ON DELETE CASCADE,seq INTEGER NOT NULL CHECK(seq >= 1),reported_at INTEGER NOT NULL,location_data TEXT NOT NULL,PRIMARY KEY(request_id,seq));

-- table n3_settings
CREATE TABLE n3_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,external_address TEXT NOT NULL DEFAULT TRUE,CHECK(singleton));

-- table nat_settings
CREATE TABLE nat_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,enabled BOOLEAN NOT NULL DEFAULT TRUE,CHECK(singleton));

-- table network_logs
CREATE TABLE "network_logs"(id INTEGER PRIMARY KEY AUTOINCREMENT,timestamp INTEGER NOT NULL,protocol TEXT NOT NULL,message_type TEXT NOT NULL,direction TEXT NOT NULL DEFAULT '',local_address TEXT NOT NULL DEFAULT '',remote_address TEXT NOT NULL DEFAULT '',raw BLOB NOT NULL,details TEXT NOT NULL DEFAULT '',radio_name TEXT NOT NULL DEFAULT '',radio_ref TEXT NOT NULL DEFAULT '');

-- table network_rules
CREATE TABLE "network_rules"(id TEXT PRIMARY KEY,policy_id TEXT NOT NULL,description TEXT NOT NULL,direction TEXT NOT NULL,remote_prefix TEXT,protocol INTEGER DEFAULT 255,port_low INTEGER DEFAULT 0,port_high INTEGER DEFAULT 0,action TEXT NOT NULL,precedence INTEGER NOT NULL,created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,FOREIGN KEY(policy_id)REFERENCES policies(id)ON DELETE CASCADE,UNIQUE(policy_id,precedence,direction));

-- table network_slices
CREATE TABLE "network_slices"(id TEXT PRIMARY KEY,sst INTEGER NOT NULL,sd TEXT,name TEXT NOT NULL UNIQUE,UNIQUE(sst,sd));

-- table operator
CREATE TABLE "operator"(id INTEGER PRIMARY KEY CHECK(id = 1),mcc TEXT NOT NULL,mnc TEXT NOT NULL,operatorCode TEXT NOT NULL,supportedTACs TEXT DEFAULT '[]',ciphering TEXT NOT NULL DEFAULT '["NEA2","NEA1"]',integrity TEXT NOT NULL DEFAULT '["NIA2","NIA1"]',spnFullName TEXT NOT NULL DEFAULT 'Ella Networks',spnShortName TEXT NOT NULL DEFAULT 'Ella',amfRegionID INTEGER NOT NULL DEFAULT 1,amfSetID INTEGER NOT NULL DEFAULT 1,clusterID TEXT NOT NULL DEFAULT '');

-- table pcscf_addresses
CREATE TABLE pcscf_addresses(priority INTEGER PRIMARY KEY CHECK(priority >= 0),address TEXT NOT NULL UNIQUE);

-- table policies
CREATE TABLE "policies"(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,profileID TEXT NOT NULL,sliceID TEXT NOT NULL,dataNetworkID TEXT NOT NULL,var5qi INTEGER NOT NULL,arp INTEGER NOT NULL,sessionAmbrUplink TEXT NOT NULL,sessionAmbrDownlink TEXT NOT NULL,isDefault INTEGER NOT NULL DEFAULT 0,preemptionCapability TEXT NOT NULL DEFAULT 'NOT_PREEMPT' CHECK(preemptionCapability IN('NOT_PREEMPT','MAY_PREEMPT')),preemptionVulnerability TEXT NOT NULL DEFAULT 'NOT_PREEMPTABLE' CHECK(preemptionVulnerability IN('NOT_PREEMPTABLE','PREEMPTABLE')),FOREIGN KEY(profileID)REFERENCES profiles(id)ON DELETE RESTRICT,FOREIGN KEY(sliceID)REFERENCES network_slices(id)ON DELETE RESTRICT,FOREIGN KEY(dataNetworkID)REFERENCES data_networks(id)ON DELETE RESTRICT,UNIQUE(profileID,sliceID,dataNetworkID));

-- table profiles
CREATE TABLE "profiles"(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,ueAmbrUplink TEXT NOT NULL,ueAmbrDownlink TEXT NOT NULL,allow4G INTEGER NOT NULL DEFAULT 1,allow5G INTEGER NOT NULL DEFAULT 1);

-- table retention_policies
CREATE TABLE "retention_policies"(id TEXT PRIMARY KEY,category TEXT NOT NULL UNIQUE,retention_days INTEGER NOT NULL CHECK(retention_days >= 1));

-- table roaming_partner_plmns
CREATE TABLE roaming_partner_plmns(partnerID TEXT NOT NULL REFERENCES roaming_partners(id)ON DELETE CASCADE,mcc TEXT NOT NULL CHECK(mcc GLOB '[0-9][0-9][0-9]'),mnc TEXT NOT NULL CHECK(mnc GLOB '[0-9][0-9]' OR mnc GLOB '[0-9][0-9][0-9]'),nrfAPIRoot TEXT CHECK(nrfAPIRoot != ''),PRIMARY KEY(mcc,mnc));

-- table roaming_partners
CREATE TABLE roaming_partners(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE CHECK(name != ''),profileID TEXT NOT NULL REFERENCES profiles(id)ON DELETE RESTRICT,seppFQDN TEXT CHECK(seppFQDN != '' AND seppFQDN = lower(seppFQDN)),seppN32cURI TEXT CHECK(seppN32cURI != ''),seppN32fURI TEXT CHECK(seppN32fURI != ''),seppTrustAnchorPEM TEXT CHECK(seppTrustAnchorPEM != ''),diameterPeerID TEXT REFERENCES diameter_peers(id)ON DELETE RESTRICT,withdrawingUntil INTEGER NOT NULL DEFAULT 0,CHECK((seppFQDN IS NULL)=(seppN32cURI IS NULL)AND(seppFQDN IS NULL)=(seppTrustAnchorPEM IS NULL)),CHECK(seppFQDN IS NOT NULL OR seppN32fURI IS NULL));

-- table routes
CREATE TABLE routes(id INTEGER PRIMARY KEY AUTOINCREMENT,destination TEXT NOT NULL,gateway TEXT NOT NULL,interface TEXT NOT NULL,metric INTEGER NOT NULL);

-- table schema_version
CREATE TABLE schema_version(id INTEGER PRIMARY KEY CHECK(id = 1),version INTEGER NOT NULL);

-- table sepp_ca
CREATE TABLE sepp_ca(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(singleton),certPEM TEXT NOT NULL CHECK(certPEM != ''),keyPEM TEXT NOT NULL CHECK(keyPEM != ''));

-- table sepp_identities
CREATE TABLE sepp_identities(seppID INTEGER PRIMARY KEY CHECK(seppID BETWEEN 1 AND 63),fqdn TEXT NOT NULL UNIQUE CHECK(fqdn != '' AND fqdn = lower(fqdn)),source TEXT NOT NULL CHECK(source IN('generated','imported')),certChainPEM TEXT NOT NULL CHECK(certChainPEM != ''),keyPEM TEXT NOT NULL CHECK(keyPEM != ''));

-- table sessions
CREATE TABLE "sessions"(id TEXT PRIMARY KEY,user_id TEXT NOT NULL,token_hash BLOB NOT NULL UNIQUE,created_at INTEGER NOT NULL DEFAULT(strftime('%s','now')),expires_at INTEGER NOT NULL,FOREIGN KEY(user_id)REFERENCES users(id)ON DELETE CASCADE);

-- table slice_data_networks
CREATE TABLE slice_data_networks(sliceID TEXT NOT NULL REFERENCES network_slices(id)ON DELETE RESTRICT,dataNetworkID TEXT NOT NULL REFERENCES data_networks(id)ON DELETE RESTRICT,nat BOOLEAN NOT NULL CHECK(nat IN(0,1)),localSwitch BOOLEAN NOT NULL CHECK(localSwitch IN(0,1)),networkInstance TEXT NOT NULL DEFAULT 'default',PRIMARY KEY(sliceID,dataNetworkID));

-- table sms_message_waiting
CREATE TABLE sms_message_waiting(imsi TEXT PRIMARY KEY REFERENCES subscribers(imsi)ON DELETE CASCADE,mcef INTEGER NOT NULL DEFAULT 0,updatedAt INTEGER NOT NULL);

-- table sms_message_waiting_centres
CREATE TABLE sms_message_waiting_centres(imsi TEXT NOT NULL REFERENCES sms_message_waiting(imsi)ON DELETE CASCADE,serviceCentre TEXT NOT NULL,PRIMARY KEY(imsi,serviceCentre));

-- table sms_settings
CREATE TABLE sms_settings(singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,smsNumber TEXT NOT NULL DEFAULT '',CHECK(singleton));

-- table sms_smsc_service_centres
CREATE TABLE "sms_smsc_service_centres"(serviceCentre TEXT PRIMARY KEY,smscId TEXT NOT NULL REFERENCES sms_smscs(id)ON DELETE CASCADE);

-- table sms_smscs
CREATE TABLE sms_smscs(id TEXT PRIMARY KEY,name TEXT NOT NULL);

-- table subscriber_framed_routes
CREATE TABLE subscriber_framed_routes(id TEXT PRIMARY KEY,imsi TEXT NOT NULL,dataNetworkID TEXT NOT NULL,prefix TEXT NOT NULL UNIQUE,FOREIGN KEY(imsi)REFERENCES subscribers(imsi)ON DELETE CASCADE,FOREIGN KEY(dataNetworkID)REFERENCES data_networks(id)ON DELETE RESTRICT);

-- table subscribers
CREATE TABLE "subscribers"(id TEXT PRIMARY KEY,imsi TEXT NOT NULL UNIQUE CHECK(length(imsi)BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),sequenceNumber TEXT NOT NULL CHECK(length(sequenceNumber)= 12),permanentKey TEXT NOT NULL CHECK(length(permanentKey)= 32),opc TEXT NOT NULL CHECK(length(opc)= 32),profileID TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',msisdn TEXT NOT NULL DEFAULT '',FOREIGN KEY(profileID)REFERENCES profiles(id)ON DELETE RESTRICT);

-- table ue_registrations
CREATE TABLE "ue_registrations"(imsi TEXT NOT NULL CHECK(length(imsi)BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),type TEXT NOT NULL,nodeID TEXT NOT NULL,purged INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL,PRIMARY KEY(imsi,type));

-- table users
CREATE TABLE "users"(id TEXT PRIMARY KEY,email TEXT NOT NULL UNIQUE,roleID INTEGER NOT NULL,hashedPassword TEXT NOT NULL);

-- table visitors
CREATE TABLE visitors(imsi TEXT PRIMARY KEY CHECK(length(imsi)BETWEEN 6 AND 15 AND imsi NOT GLOB '*[^0-9]*'),partnerID TEXT NOT NULL REFERENCES roaming_partners(id)ON DELETE RESTRICT,msisdn TEXT NOT NULL DEFAULT '' CHECK(msisdn = '' OR(length(msisdn)BETWEEN 1 AND 15 AND msisdn NOT GLOB '*[^0-9]*')),since INTEGER NOT NULL,deregisterUntil INTEGER NOT NULL DEFAULT 0);

