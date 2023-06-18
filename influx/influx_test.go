package influx_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/mu-box/pulse/influx"
	"github.com/mu-box/pulse/plexer"
)

func TestMain(m *testing.M) {
	// start influx

	// configure influx to connect to (DO NOT TEST ON PRODUCTION)
	// viper.SetDefault("influx-address", "http://localhost:8086")
	viper.SetDefault("influx-address", "http://172.28.128.4:8086")
	viper.SetDefault("aggregate-interval", 1)

	// initialize influx
	queries := []string{
		// clean influx to test with (DO NOT RUN ON PRODUCTION)
		"DROP   DATABASE statistics",
		"CREATE DATABASE statistics",
		`CREATE RETENTION POLICY one_day ON statistics DURATION 2d REPLICATION 1 DEFAULT`,
		`CREATE RETENTION POLICY one_week ON statistics DURATION 1w REPLICATION 1`,
	}
	for _, query := range queries {
		_, err := influx.Query(query)
		if err != nil {
			fmt.Printf("Failed to QUERY/INITIALIZE - '%s' skipping tests\n", err)
			os.Exit(0)
		}
	}

	rtn := m.Run()

	_, err := influx.Query("DROP DATABASE statistics")
	if err != nil {
		fmt.Println("Failed to CLEANUP - ", err)
		os.Exit(1)
	}

	os.Exit(rtn)
}
func TestInsert(t *testing.T) {
	// define fake messages
	msg1 := plexer.Message{ID: "cpu_used", Tags: []string{"cpu_not_free"}, Data: "0.34"}
	msg2 := plexer.Message{ID: "ram_used", Tags: []string{"ram_not_free"}, Data: "0.43"}
	messages := plexer.MessageSet{Tags: []string{"host:tester", "test0"}, Messages: []plexer.Message{msg1, msg2}}

	// test inserting into influx
	if err := influx.Insert(messages); err != nil {
		t.Error("Failed to INSERT messages - ", err)
	}
}

func TestQuery(t *testing.T) {
	// ensure insert worked
	response, err := influx.Query(`Select * from one_day./.*/`)
	if err != nil || len(response.Results) < 1 {
		t.Error("Failed to QUERY influx - ", err)
		t.FailNow()
	}

	cpu_used := response.Results[0].Series[0].Values[0][1]

	if cpu_used != json.Number("0.34") {
		t.Error("Failed to QUERY influx - ( BAD INSERT: expected: 0.34, got: ", cpu_used, ")")
	}
}

func TestContinuousQuery(t *testing.T) {
	// start cq checker
	go influx.KeepContinuousQueriesUpToDate()

	// give it a second to update
	time.Sleep(time.Second)

	// ensure insert worked
	response, err := influx.Query(`SHOW CONTINUOUS QUERIES`)
	if err != nil {
		t.Error("Failed to QUERY influx - ", err)
		t.FailNow()
	}

	cq := response.Results[0].Series[1].Values[0][1]
	if cq != "CREATE CONTINUOUS QUERY aggregate ON statistics BEGIN SELECT mean(cpu_used) AS cpu_used, mean(ram_used) AS ram_used INTO statistics.one_week.aggregate FROM statistics.one_day./.*/ GROUP BY time(1m), host END" {
		t.Error("Failed to UPDATE CONTINUOUS QUERY influx - mismatched create statement")
		fmt.Printf("%+q\n", cq)
	}
}
