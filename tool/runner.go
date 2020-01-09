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
	DB           []string    `json:"db"`
	Flags        []string    `json:"flags"`
	Input        []MoveData  `json:"input"`
	TestCases    []*TestCase `json:"cases"`
	Version      int         `json:"version"`
	EnvFlagFile  string      `json:"env_flag_files,omitempty"`
	Path         string      `json:"-"`
	FilesChanged []string    `json:"-"`
	FailAgain    []string    `json:"-"`
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
	RegressionDb          = flag.String("regression_db_path", "", "path to regression db")
	RegressionFlagFile    = flag.String("regression_flag", "", "path to flag file for setting regression flags")
	TestCaseConfigPattern = flag.String("test_case_config_pattern", "reg_config.json", "test case config file name")
	diffTool              = flag.String("diffTool", "./rdiff", "tool to perform diff")
	updateOldCase         = flag.Int("update_case_from_diff", 0, "whether to update test cases when diff presents")
	onTestSuitFailCmd     = flag.String("on_test_suit_fail_handler", "", "cmd to execute on test suit failure")
	outputFileChangedList = flag.String("output_file_changed", "files.changed", "file to record file that is updated")
	failAgainListFile     = flag.String("output_fail_again", "", "file to store fail again test case")
	commonFlag            = flag.String("common_server_flag", "", "common flags(newline separated) to pass to server for every run")
)

// ScanTestData scan given directory searching for test suit config file
func ScanTestData(path string) ([]*TestItem, error) {
	files := make([]string, 0, 1024)
	filepath.Walk(path, func(fp string, info os.FileInfo, err error) error {
		name := filepath.Base(fp)
		if strings.Contains(name, *TestCaseConfigPattern) {
			files = append(files, fp)
		}
		return nil
	})

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

func genUniqueFileName(file, suggest string) string {
	path := file + "." + suggest
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	for i := 0; i < 1024; i++ {
		path = fmt.Sprintf("%s.%s.%d", file, suggest, i)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}

	panic("can not create regression output dir")
}

func splitTestCase(t *TestItem, idx int) *TestItem {
	sz := len(t.TestCases)
	if sz <= 1 || idx >= sz {
		return nil
	}

	nt := *t
	nt.Path = genUniqueFileName(nt.Path, "bt")
	nt.TestCases = []*TestCase{t.TestCases[idx]}
	t.TestCases[idx] = nil
	return &nt
}

// DiffInfo diff info for display
type DiffInfo struct {
	ReqFile     string
	RspFile     string
	RspActual   string
	DiffContent string
}

// RunTestResult run test suit result
// Code 0 for failure, 1 for success
// Diff: testsuit path to diff text content
type RunTestResult struct {
	Succ int
	Fail int
	Msg  []string
	Diff map[string]DiffInfo
}

// RunTestCase run all cases from a test suit
func RunTestCase(differ, start_cmd, stop_cmd, addr string, store_dir, regression_db, regression_flag_file string, t *TestItem) ([]*TestItem, *RunTestResult) {
	util.RunCmd(stop_cmd)

	var err error
	dir := filepath.Dir(t.Path)

	ret := &RunTestResult{}
	ret.Diff = make(map[string]DiffInfo)
	ret.Msg = make([]string, 0, len(t.TestCases))

	for _, db := range t.DB {
		if len(db) > 0 && db[0] != '/' {
			db = dir + "/" + db
		}
		name := filepath.Base(db)
		err = util.CopyFile(db, regression_db+"/"+name)
		if err != nil {
			ret.Fail++

			m := fmt.Sprintf("copy regression db failed, file:%s, err:%s", db, err.Error())
			ret.Msg = append(ret.Msg, m)
			return nil, ret
		}

		m := fmt.Sprintf("done copying db data, src:%s, dst:%s", db, regression_db+name)
		ret.Msg = append(ret.Msg, m)
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
			ret.Fail++
			m := fmt.Sprintf("copying %dth input failed, src:%s, dst:%s", i, src, dst)
			ret.Msg = append(ret.Msg, m)
			return nil, ret
		}

		m := fmt.Sprintf("done copying input data, src:%s, dst:%s", src, dst)
		ret.Msg = append(ret.Msg, m)
	}

	envFlagFile := ""
	if len(t.EnvFlagFile) > 0 {
		src := t.EnvFlagFile
		if src[0] != '/' {
			src = dir + "/" + src
		}

		outdir := filepath.Dir(*RegressionFlagFile)
		dst := outdir + "/" + filepath.Base(src)

		err = util.CopyFile(src, dst)
		if err != nil {
			ret.Fail++
			m := fmt.Sprintf("copying env flag file failed, src:%s, dst:%s", src, dst)
			ret.Msg = append(ret.Msg, m)
			return nil, ret
		}

		envFlagFile = dst
		m := fmt.Sprintf("done copying env flag file, src:%s, dst:%s", src, dst)
		ret.Msg = append(ret.Msg, m)
	}

	allFlag := strings.Join(t.Flags, "\n")
	if len(*commonFlag) > 0 {
		allFlag = allFlag + "\n" + *commonFlag
	}

	flagFile, err2 := os.OpenFile(regression_flag_file, os.O_RDWR|os.O_CREATE, 0666)
	if err2 != nil {
		ret.Fail++
		m := fmt.Sprintf("open flags file failed, file:%s, err:%s", regression_flag_file, err2.Error())
		ret.Msg = append(ret.Msg, m)
		return nil, ret
	}

	flagFile.Truncate(0)
	flagFile.Seek(0, 0)
	_, err3 := flagFile.Write([]byte(allFlag))
	flagFile.Sync()
	flagFile.Close()

	if err3 != nil {
		ret.Fail++
		m := fmt.Sprintf("write to flags file failed, file:%s, err:%s", regression_flag_file, err3.Error())
		ret.Msg = append(ret.Msg, m)
		return nil, ret
	}

	envVar := make([]string, 0, 8)
	caseVer := fmt.Sprintf("CASE_VERSION=%d", t.Version)

	envVar = append(envVar, caseVer)

	if len(envFlagFile) > 0 {
		envVar = append(envVar, fmt.Sprintf("ENV_FLAG_FILE=%s", envFlagFile))
	}

	startServerCmd := strings.Join(envVar, " ") + " " + start_cmd

	_, err = util.RunCmd(startServerCmd)
	if err != nil {
		ret.Fail++
		m := fmt.Sprintf("run start cmd failed, cmd:%s, err:%s", start_cmd, err.Error())
		ret.Msg = append(ret.Msg, m)
		return nil, ret
	}

	time.Sleep(time.Duration(100) * time.Millisecond)
	m := fmt.Sprintf("start server done, cmd:%s", startServerCmd)
	ret.Msg = append(ret.Msg, m)

	res := store_dir + "/gorr.rsp.dat.tmp"

	num := 0

	fail := make([]int, 0, len(t.TestCases))
	succ := make([]*TestCase, 0, len(t.TestCases))

	newTest := make([]*TestItem, 0, len(t.TestCases))

	caseNum := len(t.TestCases)

	for i, v := range t.TestCases {
		num++
		m = fmt.Sprintf("starting to run %dth test case, name:%s, version:%d", i, v.Desc, t.Version)
		ret.Msg = append(ret.Msg, m)

		cmd := v.Runner
		if len(cmd) == 0 {
			cmd = *DefaultRunner
		}

		if cmd[0] != '/' {
			cmd = "./" + cmd
		}

		uri := ""
		if len(v.URI) > 0 {
			uri = " -uri=\"" + v.URI + "\""
		}

		cmd = caseVer + " " + cmd
		reqFile := dir + "/" + v.Req

		dt := fmt.Sprintf(" -reqType=%d -rspType=%d ", v.ReqType, v.RspType)
		cmd = cmd + " -addr=" + addr + " -input=" + reqFile + " -output=" + res + dt + uri + " -v=100 -logtostderr=true 2>&1"

		output, err := util.RunCmd(cmd)

		if err != nil {
			ret.Fail++
			fail = append(fail, i)
			m = fmt.Sprintf("\033[31m@@@@@%dth test case failed@@@@@@\033[m, request runner failed, name:%s, cmd:%s, out:%s", i, v.Desc, cmd, output)
			ret.Msg = append(ret.Msg, m)
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
                //fmt.Printf("%d-th failed, diffcmd:%s\n", i, diffCmd)
				if caseNum == 1 {
					m = fmt.Sprintf("diff failed, msg:\n%s", string(output))
					ret.Diff[t.Path] = DiffInfo{
						ReqFile:     reqFile,
						RspFile:     rspFile,
						RspActual:   res,
						DiffContent: m,
					}
					m = fmt.Sprintf("\033[31m@@@@@%dth test case failed@@@@@\033[m, name:%s, err:%v, cmd:%s, diffcmd:%s, failed before:%d, update failed:%d", i, v.Desc, err, cmd, diffCmd, v.Failed, *updateOldCase)
					ret.Msg = append(ret.Msg, m)
				}

				if *updateOldCase > 0 {
					bak := genUniqueFileName("diff", "prev")
					util.CopyFile(rspFile, bak)
					t.TestCases[i].Failed = 0
					t.FilesChanged = append(t.FilesChanged, bak)
					err = util.CopyFile(res, rspFile)
					t.FilesChanged = append(t.FilesChanged, rspFile)
					m = fmt.Sprintf("\033[31m@@@@@update test case(%d) from diff@@@@@\033[m, err:%s", i, err)
					ret.Msg = append(ret.Msg, m)
				} else if caseNum == 1 {
					t.TestCases[i].Failed = 1
				}
			} else {
				fc := fmt.Sprintf("%s/%dth-case", t.Path, i)
				t.FailAgain = append(t.FailAgain, fc)
				m = fmt.Sprintf("diff failed, msg:%s", string(output))
				ret.Diff[t.Path] = DiffInfo{
					ReqFile:     reqFile,
					RspFile:     rspFile,
					RspActual:   res,
					DiffContent: m,
				}
				m = fmt.Sprintf("\033[31m@@@@@%dth test case failed AGAIN@@@@@\033[m, name:%s, err:%v, cmd:%s", i, v.Desc, err, cmd)
				ret.Msg = append(ret.Msg, m)
			}

			ret.Fail++
			fail = append(fail, i)
			continue
		}

		ret.Succ++
		succ = append(succ, v)
		m = fmt.Sprintf("\033[32m@@@@@done running %dth test case@@@@@\033[m, name:%s, cmd:%s", i, v.Desc, cmd)
		ret.Msg = append(ret.Msg, m)
	}

	for _, idx := range fail {
		v := t.TestCases[idx]
		nt := splitTestCase(t, idx)
		if nt == nil {
			succ = append(succ, v)
			continue
		}

		var data []byte
		data, err = json.MarshalIndent(nt, "", "\t")
		if err == nil {
			err = ioutil.WriteFile(nt.Path, data, 0666)
		}

		if err != nil {
			succ = append(succ, v)
			m := fmt.Sprintf("\033[31m@@@@@@@write splitted test case failed:\033[m %s, err:%s", nt.Path, err)
			ret.Msg = append(ret.Msg, m)
		} else {
			newTest = append(newTest, nt)
			t.FilesChanged = append(t.FilesChanged, nt.Path)
			m := fmt.Sprintf("\033[31m@@@@@@@%d-th test case splitted to:\033[m %s", idx, nt.Path)
			ret.Msg = append(ret.Msg, m)
		}
	}

	if len(fail) > 0 {
		t.TestCases = succ
		data, err := json.MarshalIndent(t, "", "\t")
		if err == nil {
			ioutil.WriteFile(t.Path, data, 0666)
			t.FilesChanged = append(t.FilesChanged, t.Path)
			m := fmt.Sprintf("\033[32m@@@@@@@config for test suit is updated:\033[m %s", t.Path)
			ret.Msg = append(ret.Msg, m)
		}
	}

	return newTest, ret
}

func main() {
	flag.Parse()

	tests, err := ScanTestData(*TestDataPath)
	if err != nil {
		fmt.Printf("scan test cases failed, path:%s, err:%s\n", *TestDataPath, err.Error())
		os.Exit(33)
	}

	total := 0
	totalTestSuit := 0

	totalErr := 0
	fa := make([]string, 0, len(tests))
	files := make([]string, 0, len(tests))
	newTest := make([]*TestItem, 0, len(tests))

RUN:
	for i, t := range tests {
		testNum := len(t.TestCases)

		idx := i + totalTestSuit
		fmt.Printf("\033[32m==> start to run %d-th test suit...\033[m\n", idx)
		nt, ret := RunTestCase(*diffTool, *StartCmd, *StopCmd, *ServerAddr, *StoreDir, *RegressionDb, *RegressionFlagFile, t)

		newTest = append(newTest, nt...)

		fmt.Printf("\033[32m==> run %d-th testsuit done, succ:%d, fail:%d, run info see following:\033[m\n", idx, ret.Succ, ret.Fail)

		if ret.Fail > 0 {
			for _, m := range ret.Msg {
				fmt.Printf("%s\n", m)
			}

			totalErr += ret.Fail
			files = append(files, t.FilesChanged...)

			if testNum == 1 && len(*onTestSuitFailCmd) > 0 {
				diffFile := fmt.Sprintf("%s/diff", *StoreDir)
				for k, v := range ret.Diff {
					df := genUniqueFileName(diffFile, "ts")
					err := ioutil.WriteFile(df, []byte(v.DiffContent), 066)
					if err != nil {
						fmt.Printf("write diff file failed, err:%s, diff:%s\n", err, v)
						continue
					}

					cmd := fmt.Sprintf("TS_ID=%d TS_PATH=%s DIFF_FILE=%s REQ_FILE=%s RSP_FILE=%s RSP_ACTUAL=%s %s",
						idx, t.Path, df, v.ReqFile, v.RspFile, v.RspActual, *onTestSuitFailCmd)

					out, err := util.RunCmd(cmd)
					fmt.Printf("\033[31m==>run fail handler:%s for test:%s, err:%s, diff:%s, output:033[m\n%s\n", cmd, k, err, df, string(out))
				}
			}
		}

		if len(t.FailAgain) > 0 {
			fa = append(fa, t.FailAgain...)
		}

		total += ret.Succ + ret.Fail
	}

	totalTestSuit += len(tests)

	if len(newTest) > 0 {
		tests = newTest
		newTest = make([]*TestItem, 0, 64)
		goto RUN
	}

	data := strings.Join(files, "\n")
	fmt.Printf("\n\n\033[32m%d files updated locally:\033[m\n%s\nrecorder file:%s\n", len(files), data, *outputFileChangedList)

	err = ioutil.WriteFile(*outputFileChangedList, []byte(data), 0666)
	if err != nil {
		fmt.Printf("write file changes failed, err:%s\n", err)
	}

	if len(*failAgainListFile) > 0 && len(fa) > 0 {
		fl := strings.Join(fa, "\n")
		data = "failed-again test cases are listed following:\n" + fl
		err = ioutil.WriteFile(*failAgainListFile, []byte(data), 0666)
		if err != nil {
			fmt.Printf("write failed-again list failed, err:%s\n", err)
		}
	}

	fmt.Printf("\033[32mrun done:%d test cases\033[m, %d errors\n", total, totalErr)
	os.Exit(totalErr)
}
