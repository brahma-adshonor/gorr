package main

import (
	"gorr/util"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	//"syscall"
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
	URI     string `json:"Uri"`
	Runner  string `json:"runner"`
	Diff    int    `json:"DiffType"`
	Failed  int    `json:"Failed"`
}

type TestItem struct {
	DB           []string   `json:"db"`
	Flags        []string   `json:"flags"`
	Input        []MoveData `json:"input"`
	TestCases    []TestCase `json:"cases"`
	Version      int        `json:"version"`
	Path         string     `json:"-"`
	FilesChanged []string   `json:"-"`
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
	RegressionDb          = flag.String("gorr_db_path", "", "path to gorr db")
	RegressionFlagFile    = flag.String("gorr_flag", "", "path to flag file for setting gorr flags")
	TestCaseConfigPattern = flag.String("test_case_config_pattern", "reg_config.json", "test case config file name")
	diffTool              = flag.String("diffTool", "./rdiff", "tool to perform diff")
	updateOldCase         = flag.Int("update_case_from_diff", 0, "whether to update test cases when diff presents")
	outputFileChangedList = flag.String("file_to_store_file_changed", "files.changed", "file to record file that is updated")
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

		err = json.Unmarshal(data, &item)
		if err != nil {
			fmt.Printf("unmarshal test case config failed, skip, file:%s, err:%s\n", f, err.Error())
			return nil, fmt.Errorf("unmarshaling test cases config failed, file:%s, error:%s", f, err.Error())
		}

		item.Path = f
		ret = append(ret, &item)
	}

	fmt.Printf("scan test cases done, total:%d\n", len(ret))
	return ret, nil
}

func SetSystemDate(newTime time.Time) error {
	_, lookErr := exec.LookPath("date")
	if lookErr != nil {
		fmt.Printf("Date binary not found, cannot set system date: %s\n", lookErr.Error())
		return lookErr
	} else {
		//dateString := newTime.Format("2006-01-2 15:4:5")
		dateString := newTime.Format("2 Jan 2006 15:04:05")
		fmt.Printf("Setting system date to: %s\n", dateString)
		args := []string{"--set", dateString}
		return exec.Command("date", args...).Run()
	}
}

func RunTestCase(differ, start_cmd, stop_cmd, addr string, store_dir, gorr_db, gorr_flag_file string, t *TestItem) (int, []error) {
	util.RunCmd(stop_cmd)

	var err error
	dir := filepath.Dir(t.Path)

	for _, db := range t.DB {
		if len(db) > 0 && db[0] != '/' {
			db = dir + "/" + db
		}
		idx := strings.LastIndex(db, "/")
		if idx == -1 {
			return 0, []error{fmt.Errorf("invalid db file path, file:%s", db)}
		}
		name := db[idx:]
		err = util.CopyFile(db, gorr_db+name)
		if err != nil {
			return 0, []error{fmt.Errorf("copy gorr db failed, file:%s, err:%s", db, err.Error())}
		}

		fmt.Printf("done copying db data, src:%s, dst:%s\n", db, gorr_db+name)
	}

	for i, v := range t.Input {
		src := v.Src
		if len(src) > 0 && src[0] != '/' {
			src = dir + "/" + src
		}
		dst := v.Dst
		if len(dst) > 0 && dst[0] != '/' {
			dst = dir + "/" + dst
		}
		err = util.CopyFile(src, dst)
		if err != nil {
			return 0, []error{fmt.Errorf("copying %dth input failed, src:%s, dst:%s", i, src, dst)}
		}
		fmt.Printf("done copying input data, src:%s, dst:%s\n", src, dst)
	}

	allFlag := strings.Join(t.Flags, "\n")
	flagFile, err2 := os.OpenFile(gorr_flag_file, os.O_RDWR|os.O_CREATE, 0666)
	if err2 != nil {
		return 0, []error{fmt.Errorf("open flags file failed, file:%s, err:%s", gorr_flag_file, err2.Error())}
	}

	flagFile.Truncate(0)
	flagFile.Seek(0, 0)
	_, err3 := flagFile.Write([]byte(allFlag))
	flagFile.Sync()
	flagFile.Close()

	if err3 != nil {
		return 0, []error{fmt.Errorf("write to flags file failed, file:%s, err:%s", gorr_flag_file, err3.Error())}
	}

	caseVer := fmt.Sprintf("CASE_VERSION=%d", t.Version)

	_, err = util.RunCmd(caseVer + " " + start_cmd)
	if err != nil {
		return 0, []error{fmt.Errorf("run start cmd failed, cmd:%s, err:%s", start_cmd, err.Error())}
	}

	time.Sleep(time.Duration(100) * time.Millisecond)

	var allErr []error
	res := store_dir + "/gorr.rsp.dat.tmp"

	num := 0
	failCnt := 0
	for i, v := range t.TestCases {
		num++
		fmt.Printf("starting to run %dth test case, name:%s, version:%d\n", i, v.Desc, t.Version)

		cmd := v.Runner
		if len(cmd) == 0 {
			cmd = *DefaultRunner
		}

		if cmd[0] != '/' {
			cmd = "./" + cmd
		}

		uri := ""
		if len(v.URI) > 0 {
			uri = " -uri=" + v.URI
		}

		cmd = caseVer + " " + cmd
		reqFile := dir + "/" + v.Req

		dt := fmt.Sprintf(" -reqType=%d -rspType=%d ", v.ReqType, v.RspType)
		cmd = cmd + " -addr=" + addr + " -input=" + reqFile + " -output=" + res + dt + uri + " -v=100 -logtostderr=true 2>&1"

		output, err := util.RunCmd(cmd)

		if err != nil {
			fmt.Printf("\033[31m@@@@@%dth test case failed@@@@@@\033[m, runner failed, name:%s, cmd:%s, out:%s\n", i, v.Desc, cmd, output)
			allErr = append(allErr, fmt.Errorf("\n\033[31m@@@@@@run %dth test case failed@@@@@@\033[m, name:%s, err:%s, cmd:%s, runner output:%s", i, v.Desc, err.Error(), cmd, string(output)))
			continue
		}

		dtype := v.Diff
		if dtype == 0 {
			dtype = recorderDataTypeJSON
		}

		rspFile := dir + "/" + v.Rsp
		diffCmd := fmt.Sprintf("%s -expect=%s -actual=%s -type=%d 2>&1", differ, rspFile, res, dtype)

		output, err = util.RunCmd(diffCmd)

		if err != nil || len(output) > 0 {
			if v.Failed == 0 || *updateOldCase > 0 {
				fmt.Printf("\033[31m@@@@@%dth test case failed@@@@@\033[m, name:%s, err:%v, cmd:%s\n", i, v.Desc, err, cmd)
				m := fmt.Sprintf("diff failed, msg:%s", string(output))
				allErr = append(allErr, fmt.Errorf("\n\033[31m@@@@@@ %dth test case FAILED@@@@@@\033[m, name:%s, diffcmd:%s, detail:\n%s", i, v.Desc, diffCmd, m))
				if *updateOldCase > 0 {
					bak := rspFile + ".prev"
					util.CopyFile(rspFile, bak)
					t.FilesChanged = append(t.FilesChanged, bak)
					err = util.CopyFile(res, rspFile)
					t.FilesChanged = append(t.FilesChanged, rspFile)
					fmt.Printf("update test case(%d) from diff, err:%s\n", i, err)
				} else {
					failCnt++
					t.TestCases[i].Failed = 1
				}
			} else {
				m := fmt.Sprintf("diff failed, msg:%s", string(output))
				fmt.Printf("\033[31m@@@@@%dth test case failed AGAIN@@@@@\033[m, name:%s, err:%v, cmd:%s\ndiff:%s\n", i, v.Desc, err, cmd, m)
			}
			continue
		}

		fmt.Printf("\033[32m@@@@@done running %dth test case@@@@@\033[m, name:%s, cmd:%s\n", i, v.Desc, cmd)
	}

	if failCnt > 0 {
		data, err := json.Marshal(t)
		if err == nil {
			ioutil.WriteFile(t.Path, data, 0666)
			t.FilesChanged = append(t.FilesChanged, t.Path)
			fmt.Printf("config for test suit is updated:%s\n", t.Path)
		}
	}
	return num, allErr
}

func main() {
	flag.Parse()
	tests, err := ScanTestData(*TestDataPath)
	if err != nil {
		fmt.Printf("scan test cases failed, path:%s, err:%s\n", *TestDataPath, err.Error())
		os.Exit(33)
	}

	total := 0
	totalErr := 0
	files := make([]string, 0, 64)

	for i, t := range tests {
		fmt.Printf("starting to run %dth test suit...\n", i)
		c, errs := RunTestCase(*diffTool, *StartCmd, *StopCmd, *ServerAddr, *StoreDir, *RegressionDb, *RegressionFlagFile, t)

		if len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "\033[31m@@@@@@ %dth test suit failed\033[m, error info:\n", i)
			for _, err := range errs {
				fmt.Fprintf(os.Stderr, "\033[31m@@@@@@error msg@@@@@@\033[m:\n%s\n", err.Error())
			}

			totalErr += len(errs)
			files = append(files, t.FilesChanged...)
		}

		total += c
		fmt.Printf("\033[32mdone running %dth test suit...\033[m\n\n", i)
	}

	data := strings.Join(files, "\n")
	fmt.Printf("\033[32m%d files updated locally:\033[m\n%s\n", len(files), data)
	ioutil.WriteFile(*outputFileChangedList, []byte(data), 0666)

	fmt.Printf("\033[32mrun done:%d test cases\033[m, %d errors\n", total, totalErr)
	os.Exit(totalErr)
}
