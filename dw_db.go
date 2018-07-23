// Package models constructs data needed for render.
package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"appengine"

	"/go/context/aecontext"
	"/go/context/context"
	"/security/keystore/go/keystore"
	idpb "/se/config_ids_go_proto"
	// Used for sql.Open("mysql" dsn).
	_ "/third_party/golang/mysql/mysql"
)

const (
	passwordKeyName = "dw_db_password"
	delegatedRole   = "ll"
	dbInstance      = ""
	protoTcp        = "tcp"
	protoCloud      = "cloudsql"
	dbAddr          = "17xxx:3306"
	dbName          = "dw"
	dbUser          = "root"
	auditTable      = ""
	auditStatsTable = ""
	ticketTable     = ""
	auditSelect     = `
SELECT netblock, tags, vlan_id, building, gateway, attributes,
child_attributes, expected_value, network, audit_name,
audit_code, correlates, audit_msg, severity, state, fix_state, fix_msg, tickets, datestamp, id FROM %s
where datestamp=? and audit_name=? limit 10000`
	auditCountSelect = `
SELECT audit_name, COUNT(*) FROM %s WHERE datestamp=? GROUP BY audit_name ORDER BY audit_name`
	snapshotsSelect = `
SELECT datestamp FROM %s GROUP BY datestamp ORDER BY datestamp DESC`
	statsSelect = `
SELECT audit_name, err_count, warn_count, err_per, warn_per, datestamp FROM %s`
	fixStatsSelect = `
SELECT audit_name, autofix_count, fixed_count, datestamp FROM %s WHERE autofix_count IS NOT NULL`
	ticketsSelect = `
SELECT summary, description, audit_name, audit_code, state, datestamp, ticket_id FROM %s`
)

// AuditRecord contains the audit result data for template execution.
type AuditRecord struct {
	Netblock        string
	IPAddr          string
	Prefixlen       string
	Tags            string
	VlanID          string
	Building        string
	Gateway         string
	Attributes      string
	ChildAttributes string
	ExpectedValue   string
	Network         string
	AuditName       string
	AuditCode       string
	SuperCode       string
	Correlates      string
	AuditMsg        string
	Severity        string
	State           string
	FixState        string
	FixMsg          string
	Tickets         []string
	Datestamp       string
	ID              int
}

// StatsRecord contains audit statistical data.
type StatsRecord struct {
	AuditName string
	ErrCount  int
	WarnCount int
	ErrPer    string
	WarnPer   string
	Datestamp string
}

// FixStatsRecord contains autofix statistical data.
type FixStatsRecord struct {
	AuditName    string
	AutofixCount int
	FixedCount   int
	FixedPer     string
	Datestamp    string
}

// TicketRecord contains the ticket related data for template execution.
type TicketRecord struct {
	Summary     string
	Description string
	AuditName   string
	AuditCode   string
	State       string
	Datestamp   string
	TicketID    int
}

// sqlStore retrieves data from an SQL database.
type sqlStore struct {
	db             *sql.DB
	auditStmt      *sql.Stmt
	auditCountStmt *sql.Stmt
	snapshotsStmt  *sql.Stmt
	statsStmt      *sql.Stmt
	fixStatsStmt   *sql.Stmt
	ticketsStmt    *sql.Stmt
}

// Store defines Dragonwell SQL store interface.
type Store interface {
	AuditRecords(snapshot, auditname string) ([]*AuditRecord, error)
	AuditCount(snapshot string) (map[string]int, error)
	Snapshots() ([]string, string, string, error)
	AuditStats() (map[string][]*StatsRecord, error)
	FixStats() (map[string][]*FixStatsRecord, error)
	AuditTickets() ([]*TicketRecord, error)
	// Close releases resources associated with the store.
	Close() error
}

func openCloudSQL(ctx appengine.Context, proto, addr, instance, dbName string) (*sqlStore, error) {
	pwd, err := GetPassword(ctx, keystore.DefaultServer)
	if err != nil {
		return nil, err
	}
	pwd = strings.TrimSpace(pwd)
	// use dsn with proto "tcp" in dev/local test, use "cloudsql" on app engine.
	if appengine.IsDevAppServer() {
		proto = protoTcp
		instance = addr
	}
	dsn := fmt.Sprintf("%s:%s@%s(%s)/%s", dbUser, pwd, proto, instance, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	ctx.Infof("Connected to DB %s(%s)/%s ", proto, instance, dbName)
	s, err := initStore(db)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// NewSqlStore connects to the given db, and return Store.
func NewSqlStore(ctx appengine.Context) (Store, error) {
	s, err := openCloudSQL(ctx, protoCloud, dbAddr, dbInstance, dbName)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func initStore(db *sql.DB) (*sqlStore, error) {
	var err error
	s := &sqlStore{db: db}
	if s.auditStmt, err = s.db.Prepare(fmt.Sprintf(auditSelect, auditTable)); err != nil {
		return nil, err
	}
	if s.auditCountStmt, err = s.db.Prepare(fmt.Sprintf(auditCountSelect, auditTable)); err != nil {
		return nil, err
	}
	if s.snapshotsStmt, err = s.db.Prepare(fmt.Sprintf(snapshotsSelect, auditTable)); err != nil {
		return nil, err
	}
	if s.statsStmt, err = s.db.Prepare(fmt.Sprintf(statsSelect, auditStatsTable)); err != nil {
		return nil, err
	}
	if s.fixStatsStmt, err = s.db.Prepare(fmt.Sprintf(fixStatsSelect, auditStatsTable)); err != nil {
		return nil, err
	}
	if s.ticketsStmt, err = s.db.Prepare(fmt.Sprintf(ticketsSelect, ticketTable)); err != nil {
		return nil, err
	}
	return s, nil
}

// AuditRecords will get all audit result records by snapshot and auditname.
func (s *sqlStore) AuditRecords(snapshot, auditname string) ([]*AuditRecord, error) {
	r, err := s.auditStmt.Query(snapshot, auditname)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var netblock, auditName, auditCode, datestamp string
	var tags, vlanID, building, gateway, attributes, childAttributes, expectedValue, correlates,
		network, auditMsg, severity, state, tickets, fixState, fixMsg []byte
	var id int
	var rs []*AuditRecord
	for r.Next() {
		if err := r.Scan(&netblock, &tags, &vlanID, &building, &gateway, &attributes,
			&childAttributes, &expectedValue, &network,
			&auditName, &auditCode, &correlates, &auditMsg, &severity, &state, &fixState, &fixMsg, &tickets,
			&datestamp, &id); err != nil {
			return nil, err
		}
		ipFields := strings.Split(netblock, "/")
		superCode := strings.Split(auditCode, "_")
		rs = append(rs, &AuditRecord{
			netblock, ipFields[0], ipFields[1], string(tags), string(vlanID), string(building), string(gateway),
			string(attributes), string(childAttributes), string(expectedValue), string(network), auditName,
			auditCode, superCode[0], string(correlates), string(auditMsg), string(severity), string(state), string(fixState), string(fixMsg), strings.Split(string(tickets), ","), datestamp, id,
		})
	}

	return rs, nil
}

// AuditCount fetches result count of audits by snapshot.
func (s *sqlStore) AuditCount(snapshot string) (map[string]int, error) {
	r, err := s.auditCountStmt.Query(snapshot)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var auditName string
	var count int
	acMap := make(map[string]int)
	for r.Next() {
		if err := r.Scan(&auditName, &count); err != nil {
			return nil, err
		}
		acMap[auditName] = count
	}

	return acMap, nil
}

// Snapshots fetches all available audit datestamp snapshots.
func (s *sqlStore) Snapshots() ([]string, string, string, error) {
	r, err := s.snapshotsStmt.Query()
	if err != nil {
		return nil, "", "", err
	}
	defer r.Close()
	var datestamp, minDate, maxDate string
	var snapshots []string
	for r.Next() {
		if err := r.Scan(&datestamp); err != nil {
			return nil, "", "", err
		}
		snapshots = append(snapshots, datestamp)
	}
	if len(snapshots) > 0 {
		maxDate = snapshots[0]
		minDate = snapshots[len(snapshots)-1]
	}

	return snapshots, minDate, maxDate, nil
}

// AuditStats fetches statistical data of all audits.
func (s *sqlStore) AuditStats() (map[string][]*StatsRecord, error) {
	r, err := s.statsStmt.Query()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var auditName, datestamp, errPer, warnPer string
	var errCount, warnCount int
	var errPerf, warnPerf float64
	asMap := make(map[string][]*StatsRecord)
	for r.Next() {
		if err := r.Scan(&auditName, &errCount, &warnCount, &errPerf, &warnPerf, &datestamp); err != nil {
			return nil, err
		}
		errPer = fmt.Sprintf("%.4f", errPerf)
		warnPer = fmt.Sprintf("%.4f", warnPerf)
		asMap[auditName] = append(asMap[auditName], &StatsRecord{
			auditName, errCount, warnCount, errPer, warnPer, datestamp,
		})
	}
	return asMap, nil
}

// FixStats fetches statistical data of autofix.
func (s *sqlStore) FixStats() (map[string][]*FixStatsRecord, error) {
	r, err := s.fixStatsStmt.Query()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var auditName, datestamp, fixedPer string
	var autofixCount, fixedCount int
	totalMap := make(map[string]int)
	fixedMap := make(map[string]int)
	fsMap := make(map[string][]*FixStatsRecord)
	for r.Next() {
		if err := r.Scan(&auditName, &autofixCount, &fixedCount, &datestamp); err != nil {
			return nil, err
		}
		if autofixCount > 0 {
			fixedPer = fmt.Sprintf("%.4f", float64(fixedCount)/float64(autofixCount))
			totalMap[datestamp] += autofixCount
			fixedMap[datestamp] += fixedCount
			fsMap[auditName] = append(fsMap[auditName], &FixStatsRecord{
				AuditName:    auditName,
				AutofixCount: autofixCount,
				FixedCount:   fixedCount,
				FixedPer:     fixedPer,
				Datestamp:    datestamp,
			})
		}
	}
	for k := range totalMap {
		fixedPer = fmt.Sprintf("%.4f", float64(fixedMap[k])/float64(totalMap[k]))
		fsMap["total"] = append(fsMap["total"], &FixStatsRecord{
			"total", totalMap[k], fixedMap[k], fixedPer, k,
		})
	}
	return fsMap, nil
}

// AuditTickets fetches tickets of all audits.
func (s *sqlStore) AuditTickets() ([]*TicketRecord, error) {
	r, err := s.ticketsStmt.Query()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var summary, desc, auditName, auditCode, state, datestamp string
	var ticketID int
	var rs []*TicketRecord
	for r.Next() {
		if err := r.Scan(&summary, &desc, &auditName, &auditCode, &state, &datestamp, &ticketID); err != nil {
			return nil, err
		}

		rs = append(rs, &TicketRecord{
			summary, desc, auditName, auditCode, state, datestamp, ticketID,
		})
	}
	return rs, nil
}

// Close releases the SQL database.
func (s *sqlStore) Close() error {
	return s.db.Close()
}

// GetPassword gets DB passwd from keystore.
func GetPassword(ctx appengine.Context, server string) (string, error) {
	// Activating Delegation on the Stubby Service Proxy for AppEngine App.
	var cancel context.CancelFunc
	clientOptions := &keystore.ClientOptions{DelegatedRole: delegatedRole}
	c := aecontext.WithAppEngine(context.TODO(), ctx)
	if appengine.IsDevAppServer() {
		// keystore-dev has been slow to respond, so give it a little extra time.
		c, cancel = context.WithTimeout(c, time.Second*10)
		defer cancel()
		clientOptions = nil
	}

	ctx.Infof("clientOptions: %v", clientOptions)
	// KeystoreConfigIds_NETOPS_CORP is 70890.
	keystoreClient, err := keystore.NewClient(server, int32(idpb.KeystoreConfigIds_NETOPS_CORP), clientOptions)
	if err != nil {
		return "", err
	}
	defer keystoreClient.Close()

	// get RawServiceKey.
	pwd, err := keystoreClient.RawServiceKey(c, passwordKeyName)
	if err != nil {
		return "", err
	}
	return pwd, nil
}
