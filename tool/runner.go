package main

import (
	"gorr/util"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MoveData struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

type TestCase struct {
	Req     string `json:"req"`
	Rsp     string `json:"rsp"`
	ReqType int    `json:"ReqType"`
	RspType int    `json:"RspType"`
	Desc    string `json:"Desc"`
	Runner  string `json:"runner"`
	Diff    int    `json:"DiffType"`
}

type TestItem struct {
	DB        []string   `json:"db"`
	Flags     []string   `json:"flags"`
	Input     []MoveData `json:"input"`
	TestCases []TestCase `json:"cases"`
}

const (
	recorderDataTypeUnknown  = 23
	recorderDataTypeJSON     = 24
	recorderDataTypePbText   = 25
	recorderDataTypePbBinary = 26
)

var (
	ServerAddr            = flag.String("server_addr", "", "server address")
	DefaultRunner         = flag.String("runner", "", "run to issue request")
	StoreDir              = flag.String("tmp_store_dir", "/tmp", "server address")
	TestDataPath          = flag.String("test_case_dir", "", "directory for all test cases")
	StartCmd              = flag.String("server_start_cmd", "", "cmd to start target server")
	StopCmd               = flag.String("server_stop_cmd", "", "cmd to stop target server")
	ServerLog             = flag.String("server_log_file", "", "log file path to server log")
	RegressionDb          = flag.String("regression_db_path", "", "path to regression db")
	RegressionFlagFile    = flag.String("regression_flag", "", "path to flag file for setting regression flags")
	TestCaseConfigPattern = flag.String("test_case_config_pattern", "reg_config.json", "test case config file name")
	diffTool              = flag.String("diffTool", "./rdiff", "tool to perform diff")
	updateOldCase         = flag.Int("update_case_from_diff", 0, "whether to update test cases when diff presents")
)

func ScanTestData(path string) ([]*TestItem, error) {
	pattern := path + "/*/" + *TestCaseConfigPattern

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("iterating cases dir failed, err:%s", err.Error())
	}

	ret := make([]*TestItem, 0, len(files))
	for _, f := range files {
		var item TestItem

		fmt.Printf("scanning test case file:%s\n", f)

		data, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Printf("read file failed, skip, path:%s, err:%s\n", f, err.Error())
			continue
		}

		dir := filepath.Dir(f)
		err = json.Unmarshal(data, &item)
		if err != nil {
			fmt.Printf("unmarshal test case config failed, skip, file:%s, err:%s\n", f, err.Error())
			return nil, fmt.Errorf("unmarshaling test cases config failed, file:%s, error:%s", f, err.Error())
		}

		for i, db := range item.DB {
			item.DB[i] = dir + "/" + db
		}

		for i := range item.Input {
			item.Input[i].Src = dir + "/" + item.Input[i].Src
		}

		for i := range item.TestCases {
			if len(item.TestCases[i].Runner) == 0 {
				item.TestCases[i].Runner = *DefaultRunner
			}

			item.TestCases[i].Req = dir + "/" + item.TestCases[i].Req
			item.TestCases[i].Rsp = dir + "/" + item.TestCases[i].Rsp
		}

		ret = append(ret, &item)
	}

	fmt.Printf("scan test cases done, total:%d\n", len(ret))
	return ret, nil
}

func RunTestCase(differ, start_cmd, stop_cmd, addr string, store_dir, regression_db, regression_flag_file string, t *TestItem) (int, []error) {
	util.RunCmd(stop_cmd)

	var err error
	for _, db := range t.DB {
		idx := strings.LastIndex(db, "/")
		if idx == -1 {
			return 0, []error{fmt.Errorf("invalid db file path, file:%s", db)}
		}
		name := db[idx:]
		err = util.CopyFile(db, regression_db+name)
		if err != nil {
			return 0, []error{fmt.Errorf("copy regression db failed, file:%s, err:%s", db, err.Error())}
		}

		fmt.Printf("done copying db data, src:%s, dst:%s\n", db, regression_db+name)
	}

	for i, v := range t.Input {
		err = util.CopyFile(v.Src, v.Dst)
		if err != nil {
			return 0, []error{fmt.Errorf("copying %dth input failed, src:%s, dst:%s", i, v.Src, v.Dst)}
		}
		fmt.Printf("done copying input data, src:%s, dst:%s\n", v.Src, v.Dst)
	}

	all_flag := strings.Join(t.Flags, "\n")
	flag_file, err2 := os.OpenFile(regression_flag_file, os.O_RDWR|os.O_CREATE, 0666)
	if err2 != nil {
		return 0, []error{fmt.Errorf("open flags file failed, file:%s, err:%s", regression_flag_file, err2.Error())}
	}

	flag_file.Truncate(0)
	flag_file.Seek(0, 0)
	_, err3 := flag_file.Write([]byte(all_flag))
	flag_file.Sync()
	flag_file.Close()

	if err3 != nil {
		return 0, []error{fmt.Errorf("write to flags file failed, file:%s, err:%s", regression_flag_file, err3.Error())}
	}

	_, err = util.RunCmd(start_cmd)
	if err != nil {
		return 0, []error{fmt.Errorf("run start cmd failed, cmd:%s, err:%s", start_cmd, err.Error())}
	}

	time.Sleep(time.Duration(100) * time.Millisecond)

	var all_err []error
	res := store_dir + "/regression.rsp.dat.tmp"

	num := 0
	for i, v := range t.TestCases {
		num++
		fmt.Printf("starting to run %dth test case, name:%s\n", i, v.Desc)
		cmd := v.Runner
		if cmd[0] != '/' {
			cmd = "./" + cmd
		}

		dt := fmt.Sprintf(" -reqType=%d -rspType=%d ", v.ReqType, v.RspType)
		cmd = cmd + " -addr=" + addr + " -input=" + v.Req + " -output=" + res + dt + " -v=100 -logtostderr=true 2>&1"
		output, err := util.RunCmd(cmd)

		if err != nil {
			fmt.Printf("\033[31m@@@@@%dth test case failed@@@@@@\033[m, runner failed, name:%s, cmd:%s, out:%s\n", i, v.Desc, cmd, output)
			all_err = append(all_err, fmt.Errorf("\n\033[31m@@@@@@run %dth test case failed@@@@@@\033[m, name:%s, err:%s, cmd:%s, runner output:%s", i, v.Desc, err.Error(), cmd, string(output)))
			continue
		}

		dtype := v.Diff
		if dtype == 0 {
			dtype = recorderDataTypeJSON
		}

		diffCmd := fmt.Sprintf("%s -expect=%s -actual=%s -type=%d 2>&1", differ, v.Rsp, res, dtype)
		output, err = util.RunCmd(diffCmd)

		if err != nil || len(output) > 0 {
			fmt.Printf("\033[31m@@@@@%dth test case failed@@@@@\033[m, name:%s, err:%v, cmd:%s\n", i, v.Desc, err, cmd)
			m := fmt.Sprintf("diff failed, msg:%s", string(output))
			all_err = append(all_err, fmt.Errorf("\n\033[31m@@@@@@ %dth test case FAILED@@@@@@\033[m, name:%s, diffcmd:%s, detail:\n%s", i, v.Desc, diffCmd, m))
			if *updateOldCase > 0 {
				err = util.CopyFile(res, v.Rsp)
				fmt.Printf("update test case(%d) from diff, err:%s\n", i, err)
			}
			continue
		}

		fmt.Printf("\033[32m@@@@@done running %dth test case@@@@@\033[m, name:%s, cmd:%s\n", i, v.Desc, cmd)
	}

	return num, all_err
}

func main() {
	flag.Parse()
	tests, err := ScanTestData(*TestDataPath)
	if err != nil {
		fmt.Printf("scan test cases failed, path:%s, err:%s\n", *TestDataPath, err.Error())
		os.Exit(33)
	}

	total := 0
	total_err := 0
	for i, t := range tests {
		fmt.Printf("starting to run %dth test suit...\n", i)
		c, errs := RunTestCase(*diffTool, *StartCmd, *StopCmd, *ServerAddr, *StoreDir, *RegressionDb, *RegressionFlagFile, t)

		if len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "\033[31m@@@@@@ %dth test suit failed\033[m, error info:\n", i)
			for _, err := range errs {
				fmt.Fprintf(os.Stderr, "\033[31m@@@@@@error msg@@@@@@\033[m:\n%s\n", err.Error())
			}

			total_err += len(errs)
			if len(*ServerLog) > 0 {
				info, err2 := ioutil.ReadFile(*ServerLog)
				if err2 == nil {
					fmt.Fprintf(os.Stderr, "@@@@@@@@@@@@@@@@@@@server log info:@@@@@@@@@@@@@@@@\n%s\n@@@@@@@@@@@@@@@@@@@@@@@@@@\n", string(info))
				} else {
					fmt.Fprintf(os.Stderr, "@@@@@@@@@@@@@@@@@@@read server log failed, file:%s, err:%s@@@@@@@@@@@@@@@@@@@@@@@@\n", *ServerLog, err2.Error())
				}
			} else {
				fmt.Fprintf(os.Stderr, "@@@@@@@@@@@@@@no server log specified@@@@@@@@@@@\n")
			}
		}

		total += c
		fmt.Printf("\033[32mdone running %dth test suit...\033[m\n\n", i)
	}

	fmt.Printf("\033[32mrun done:%d test cases\033[m, %d errors\n", total, total_err)
	os.Exit(total_err)
}
