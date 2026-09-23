package rds

import "github.com/aws/aws-sdk-go-v2/service/rds/types"

// dbSummary is one row of `ctl aws rds ls` — either a standalone DB
// instance or an Aurora DB cluster, distinguished by kind since RDS exposes
// them as two separate resource types with their own identifier namespace.
type dbSummary struct {
	identifier *string
	kind       string // "instance" or "cluster"
	engine     *string
	status     *string
}

type dbListOutput struct {
	items []*dbSummary
}

// dbDefinition is `ctl aws rds def <identifier>`'s output: the
// deterministic instance/cluster configuration plus a Hypothesis-grade AI
// narration of it. aiSummary is additive, never a replacement for the raw
// fields below — if empty, aiSummaryUnavailable explains why (ADR-010),
// mirroring every other *Definition type in this codebase.
type dbDefinition struct {
	identifier          *string
	kind                string
	engine              *string
	engineVersion       *string
	status              *string
	instanceClass       *string // instance only
	allocatedStorage    *int32
	multiAZ             *bool
	storageEncrypted    *bool
	publiclyAccessible  *bool
	backupRetentionDays *int32
	endpoint            *string

	aiSummary            string
	aiSummaryUnavailable string
}

func newDBSummaryFromInstance(inst types.DBInstance) *dbSummary {
	return &dbSummary{identifier: inst.DBInstanceIdentifier, kind: "instance", engine: inst.Engine, status: inst.DBInstanceStatus}
}

func newDBSummaryFromCluster(c types.DBCluster) *dbSummary {
	return &dbSummary{identifier: c.DBClusterIdentifier, kind: "cluster", engine: c.Engine, status: c.Status}
}

func newDBDefinitionFromInstance(inst types.DBInstance) *dbDefinition {
	def := &dbDefinition{
		identifier:          inst.DBInstanceIdentifier,
		kind:                "instance",
		engine:              inst.Engine,
		engineVersion:       inst.EngineVersion,
		status:              inst.DBInstanceStatus,
		instanceClass:       inst.DBInstanceClass,
		allocatedStorage:    inst.AllocatedStorage,
		multiAZ:             inst.MultiAZ,
		storageEncrypted:    inst.StorageEncrypted,
		publiclyAccessible:  inst.PubliclyAccessible,
		backupRetentionDays: inst.BackupRetentionPeriod,
	}
	if inst.Endpoint != nil {
		def.endpoint = inst.Endpoint.Address
	}
	return def
}

func newDBDefinitionFromCluster(c types.DBCluster) *dbDefinition {
	return &dbDefinition{
		identifier:          c.DBClusterIdentifier,
		kind:                "cluster",
		engine:              c.Engine,
		engineVersion:       c.EngineVersion,
		status:              c.Status,
		allocatedStorage:    c.AllocatedStorage,
		multiAZ:             c.MultiAZ,
		storageEncrypted:    c.StorageEncrypted,
		publiclyAccessible:  c.PubliclyAccessible,
		backupRetentionDays: c.BackupRetentionPeriod,
		endpoint:            c.Endpoint,
	}
}

func (def *dbDefinition) SetAISummary(summary string) *dbDefinition {
	def.aiSummary = summary
	return def
}

func (def *dbDefinition) SetAISummaryUnavailable(reason string) *dbDefinition {
	def.aiSummaryUnavailable = reason
	return def
}
