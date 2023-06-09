package v1

import (
	"context"
	"errors"
	"fmt"

	"github.com/influxdata/flux"
	"github.com/influxdata/flux/execute"
	"github.com/influxdata/flux/memory"
	"github.com/influxdata/flux/plan"
	"github.com/influxdata/flux/runtime"
	"github.com/influxdata/flux/values"
	"github.com/influxdata/influxdb/flux/stdlib/influxdata/influxdb"
	"github.com/influxdata/influxdb/services/meta"
	"github.com/influxdata/influxql"
)

const DatabasesKind = "influxDBDatabases"

type DatabasesOpSpec struct {
}

func init() {
	databasesSignature := runtime.MustLookupBuiltinType("influxdata/influxdb/v1", "databases")
	runtime.ReplacePackageValue("influxdata/influxdb/v1", "databases", flux.MustValue(flux.FunctionValue(DatabasesKind, createDatabasesOpSpec, databasesSignature)))
	plan.RegisterProcedureSpec(DatabasesKind, newDatabasesProcedure, DatabasesKind)
}

func createDatabasesOpSpec(args flux.Arguments, a *flux.Administration) (flux.OperationSpec, error) {
	spec := new(DatabasesOpSpec)
	return spec, nil
}

func newDatabasesOp() flux.OperationSpec {
	return new(DatabasesOpSpec)
}

func (s *DatabasesOpSpec) Kind() flux.OperationKind {
	return DatabasesKind
}

type DatabasesProcedureSpec struct {
	plan.DefaultCost
}

func newDatabasesProcedure(qs flux.OperationSpec, pa plan.Administration) (plan.ProcedureSpec, error) {
	_, ok := qs.(*DatabasesOpSpec)
	if !ok {
		return nil, fmt.Errorf("invalid spec type %T", qs)
	}

	return &DatabasesProcedureSpec{}, nil
}

func (s *DatabasesProcedureSpec) Kind() plan.ProcedureKind {
	return DatabasesKind
}

func (s *DatabasesProcedureSpec) Copy() plan.ProcedureSpec {
	ns := new(DatabasesProcedureSpec)
	return ns
}

func init() {
	execute.RegisterSource(DatabasesKind, createDatabasesSource)
}

type DatabasesDecoder struct {
	deps      *influxdb.StorageDependencies
	databases []meta.DatabaseInfo
	user      meta.User
	alloc     memory.Allocator
}

func (bd *DatabasesDecoder) Connect(ctx context.Context) error {
	return nil
}

func (bd *DatabasesDecoder) Fetch(ctx context.Context) (bool, error) {
	bd.databases = bd.deps.MetaClient.Databases()
	return false, nil
}

func (bd *DatabasesDecoder) Decode(ctx context.Context) (flux.Table, error) {
	kb := execute.NewGroupKeyBuilder(nil)
	kb.AddKeyValue("organizationID", values.NewString(""))
	gk, err := kb.Build()
	if err != nil {
		return nil, err
	}

	b := execute.NewColListTableBuilder(gk, bd.alloc)
	if _, err := b.AddCol(flux.ColMeta{
		Label: "organizationID",
		Type:  flux.TString,
	}); err != nil {
		return nil, err
	}
	if _, err := b.AddCol(flux.ColMeta{
		Label: "databaseName",
		Type:  flux.TString,
	}); err != nil {
		return nil, err
	}
	if _, err := b.AddCol(flux.ColMeta{
		Label: "retentionPolicy",
		Type:  flux.TString,
	}); err != nil {
		return nil, err
	}
	if _, err := b.AddCol(flux.ColMeta{
		Label: "retentionPeriod",
		Type:  flux.TInt,
	}); err != nil {
		return nil, err
	}
	if _, err := b.AddCol(flux.ColMeta{
		Label: "default",
		Type:  flux.TBool,
	}); err != nil {
		return nil, err
	}
	if _, err := b.AddCol(flux.ColMeta{
		Label: "bucketId",
		Type:  flux.TString,
	}); err != nil {
		return nil, err
	}

	var hasAccess func(db string) bool
	if bd.user == nil {
		hasAccess = func(db string) bool {
			return true
		}
	} else {
		hasAccess = func(db string) bool {
			return bd.deps.Authorizer.AuthorizeDatabase(bd.user, influxql.ReadPrivilege, db) == nil ||
				bd.deps.Authorizer.AuthorizeDatabase(bd.user, influxql.WritePrivilege, db) == nil
		}
	}

	for _, db := range bd.databases {
		if hasAccess(db.Name) {
			for _, rp := range db.RetentionPolicies {
				_ = b.AppendString(0, "")
				_ = b.AppendString(1, db.Name)
				_ = b.AppendString(2, rp.Name)
				_ = b.AppendInt(3, rp.Duration.Nanoseconds())
				_ = b.AppendBool(4, db.DefaultRetentionPolicy == rp.Name)
				_ = b.AppendString(5, "")
			}
		}
	}

	return b.Table()
}

func (bd *DatabasesDecoder) Close() error {
	return nil
}

func createDatabasesSource(prSpec plan.ProcedureSpec, dsid execute.DatasetID, a execute.Administration) (execute.Source, error) {
	_, ok := prSpec.(*DatabasesProcedureSpec)
	if !ok {
		return nil, fmt.Errorf("invalid spec type %T", prSpec)
	}
	deps := influxdb.GetStorageDependencies(a.Context())
	var user meta.User
	if deps.AuthEnabled {
		user = meta.UserFromContext(a.Context())
		if user == nil {
			return nil, errors.New("createDatabasesSource: no user")
		}
	}
	bd := &DatabasesDecoder{deps: &deps, alloc: a.Allocator(), user: user}
	return execute.CreateSourceFromDecoder(bd, dsid, a)
}
