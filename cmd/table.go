// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

const (
	tablePrefix  = "tables/"
	regionSuffix = "regions"
	usageSurffix = "disk-usage"
)

const (
	pdAddrFlagName = "pd-addr"
	tableIdName    = "table-id"
	indexIdName    = "index-id"
)

var (
	tableDB    string
	tableTable string
	pdAddr     string
	tableId    uint64
	indexId    uint64
)

// tableCmd represents the table command
var tableRootCmd = &cobra.Command{
	Use:   "table",
	Short: "Table information",
	Long:  `tidb-ctl table`,
}

func init() {
	scatterRangeCmd.Flags().StringVarP(&pdAddr, pdAddrFlagName, "p", "", "remote pd address")
	scatterRangeCmd.Flags().Uint64Var(&tableId, tableIdName, 0, "table id")
	scatterRangeCmd.Flags().Uint64Var(&indexId, indexIdName, 0, "index id")
	requiredFlags := []string{
		pdAddrFlagName, tableIdName,
	}
	for _, v := range requiredFlags {
		if err := scatterRangeCmd.MarkFlagRequired(v); err != nil {
			fmt.Printf("can not mark flag, flag %s is not found", v)
			return
		}
	}
	tableRootCmd.AddCommand(regionCmd, diskUsageCmd, scatterRangeCmd)
	tableRootCmd.PersistentFlags().StringVarP(&tableDB, dbFlagName, "d", "", "database name")
	tableRootCmd.PersistentFlags().StringVarP(&tableTable, tableFlagName, "t", "", "table name")
	if err := tableRootCmd.MarkPersistentFlagRequired(dbFlagName); err != nil {
		fmt.Printf("can not mark required flag, flag %s is not found", dbFlagName)
		return
	}
	if err := tableRootCmd.MarkPersistentFlagRequired(tableFlagName); err != nil {
		fmt.Printf("can not mark required flag, flag %s is not found", tableFlagName)
		return
	}
}

var regionCmd = &cobra.Command{
	Use:   regionSuffix,
	Short: "region info of table",
	Long:  "tidb-ctl table region --database(-d) [database name] --table(-t) [table name]",
	RunE:  getTableRegion,
}

func getTableRegion(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("too many arguments")
	}
	return httpPrint(tablePrefix + tableDB + "/" + tableTable + "/" + regionSuffix)
}

var diskUsageCmd = &cobra.Command{
	Use:   usageSurffix,
	Short: "disk usage of table",
	Long:  "tidb-ctl table disk-usage --database(-d) [database name] --table(-t) [table name]",
	RunE:  getTableDiskUsage,
}

func getTableDiskUsage(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("too many arguments")
	}
	return httpPrint(tablePrefix + tableDB + "/" + tableTable + "/" + usageSurffix)
}

const scatterRangeName = "scatter-range"

var scatterRangeCmd = &cobra.Command{
	Use:   scatterRangeName,
	Short: "scatter range of table",
	Long:  fmt.Sprintf("tidb-ctl table %s --pd-addr(-p) [pd address] --table-id [table id] --database(-d) [database name] --table(-t) [table name]", scatterRangeName),
	RunE:  scatterRange,
}

func scatterRange(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("too many arguments")
	}

	if cmd.Flag(tableIdName).Changed {
		var startKey, endKey []byte
		var rangeName string
		if cmd.Flag(indexIdName).Changed {
			startKey, endKey = tablecodec.GetTableIndexKeyRange(int64(tableId), int64(indexId))
			startKey = codec.EncodeBytes([]byte{}, startKey)
			endKey = codec.EncodeBytes([]byte{}, endKey)
			rangeName = fmt.Sprintf("%s.%s-%d-%d", tableDB, tableTable, tableId, indexId)
		} else {
			startKey, endKey = tablecodec.GetTableHandleKeyRange(int64(tableId))
			startKey = codec.EncodeBytes([]byte{}, startKey)
			endKey = codec.EncodeBytes([]byte{}, endKey)
			rangeName = fmt.Sprintf("%s.%s-%d", tableDB, tableTable, tableId)
		}

		input := map[string]string{
			"name":       "scatter-range",
			"start_key":  url.QueryEscape(string(startKey)),
			"end_key":    url.QueryEscape(string(endKey)),
			"range_name": rangeName,
		}
		v, err := json.Marshal(input)
		if err != nil {
			return err
		}
		scheduleURL := fmt.Sprintf("http://%s/pd/api/v1/schedulers", pdAddr)
		resp, err := http.Post(scheduleURL, "application/json", bytes.NewBuffer(v))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return errors.Errorf("request error: %s", string(body))
		}
	}

	return nil
}
