package sql

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/marcuswestin/fun-go/errs"
	"github.com/marcuswestin/fun-go/random"
)

type ShardSet struct {
	username string
	password string
	host     string
	port     int
	// tls has to be a string b/c we need to be able to pass in `skip-verify`
	// more info: https://github.com/go-sql-driver/mysql#tls
	tls             string
	dbNamePrefix    string
	numShards       int
	maxShards       int
	maxConns        int
	shards          []*Shard
	beginEndHandler func() (func(), error)
	metricsHandler  func(query string, shardName string) func()
	log             *log.Logger
}

func WithBeginEndHandler(handler func() (func(), error)) func(*ShardSet) {
	return func(s *ShardSet) {
		s.beginEndHandler = handler
	}
}
func WithMetricsHandler(handler func(query string, shardName string) func()) func(*ShardSet) {
	return func(s *ShardSet) {
		s.metricsHandler = handler
	}
}
func WithLogger(logger *log.Logger) func(*ShardSet) {
	return func(s *ShardSet) {
		s.log = logger
	}
}

func WithTLS(tls string) func(*ShardSet) {
	return func(s *ShardSet) {
		s.tls = tls
	}
}

func NewShardSet(username string, password string, host string, port int, dbNamePrefix string, numShards int, maxShards int, maxConns int, options ...func(*ShardSet)) *ShardSet {
	s := &ShardSet{
		username:     username,
		password:     password,
		host:         host,
		port:         port,
		dbNamePrefix: dbNamePrefix,
		numShards:    numShards,
		maxShards:    maxShards,
		maxConns:     maxConns,
		log:          log.New(ioutil.Discard, "", 0),
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *ShardSet) Connect() (err errs.Err) {
	s.shards = make([]*Shard, s.numShards)
	for i := 0; i < s.numShards; i++ {
		err = s.addShard(i)
		if err != nil {
			return
		}
	}
	return
}

func (s *ShardSet) Shard(id int64) *Shard {
	if id == 0 {
		panic("Bad shard index id 0")
	}
	shardIndex := ((id - 1) % int64(s.maxShards)) // 1->0, 2->1, 3->2 ..., 65000->65000, 65001->0, 65002->1
	return s.shards[shardIndex]
}

func (s *ShardSet) All() []*Shard {
	all := make([]*Shard, len(s.shards))
	for i, shard := range s.shards {
		all[i] = shard
	}
	return all
}

func (s *ShardSet) RandomShard() *Shard {
	return s.shards[random.Between(0, len(s.shards))]
}

func (s *ShardSet) addShard(i int) (err errs.Err) {
	autoIncrementOffset := i + 1
	dbName := fmt.Sprint(s.dbNamePrefix, autoIncrementOffset)
	s.shards[i], err = newShard(s, dbName, autoIncrementOffset)
	if err != nil {
		return
	}
	return
}

func newShard(s *ShardSet, dbName string, autoIncrementOffset int) (*Shard, errs.Err) {
	tls := s.tls
	if tls == "" {
		tls = "false"
	}
	connVars := ConnVariables{
		"autocommit":               "true",
		"clientFoundRows":          "true",
		"charset":                  "utf8mb4",
		"collation":                "utf8_unicode_ci",
		"auto_increment_increment": strconv.Itoa(s.maxShards),
		"auto_increment_offset":    strconv.Itoa(autoIncrementOffset),
		"sql_mode":                 "STRICT_ALL_TABLES",
		"tls":                      tls,
	}

	db, err := dbOpener(s.username, s.password, dbName, s.host, s.port, connVars)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(s.maxConns)
	db.SetMaxIdleConns(s.maxConns / 2)
	db.SetConnMaxLifetime(30 * time.Minute)
	stdErr := db.Ping()
	if stdErr != nil {
		return nil, errs.Wrap(stdErr, nil)
	}
	return &Shard{DBName: dbName, db: db, sqlConn: db, BeginEndHandler: s.beginEndHandler, MetricsHandler: s.metricsHandler, log: s.log}, nil
}

func SetOpener(opener Opener) {
	if dbOpener != nil {
		panic("Opener already set - did you import two driver adapters?")
	}
	dbOpener = opener
}

type ConnVariables map[string]string

func (connVars ConnVariables) Join(sep string) string {
	kvps := make([]string, len(connVars))
	i := 0
	for param, val := range connVars {
		kvps[i] = param + "=" + val
		i += 1
	}
	return strings.Join(kvps, sep)
}

type Opener func(username, password, dbName, host string, port int, connVars ConnVariables) (*sql.DB, errs.Err)

var dbOpener Opener
