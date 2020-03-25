#! /bin/bash

set -e

# TS_ID/TS_PATH/DIFF_FILE/REQ_FILE/RSP_FILE/RSP_ACTUAL

LOG_FILE="fail.info.for.test.${TS_ID}.txt"
LOG_PATH=${INSTALL_DIR}/regression/${LOG_FILE}
FAIL_LIST=${INSTALL_DIR}/regression/fail.list.log

DiffData="$(cat $DIFF_FILE)"
ReqData="$(cat $REQ_FILE)"
RspData="$(cat $RSP_FILE)"
RspActual="$(cat $RSP_ACTUAL)"

echo "rsp diff:" >>${LOG_PATH}
echo "$DiffData" >>${LOG_PATH}
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@" >>${LOG_PATH}

echo "req data:" >>${LOG_PATH}
echo "$ReqData" >>${LOG_PATH}
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@" >>${LOG_PATH}

echo "rsp data:" >>${LOG_PATH}
echo "$RspData" >>${LOG_PATH}
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@" >>${LOG_PATH}

echo "actual rsp data:" >>${LOG_PATH}
echo "$RspActual" >>${LOG_PATH}
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@" >>${LOG_PATH}

echo "server log:" >>${LOG_PATH}
cat ${SERVER_LOG} >>${LOG_PATH}

echo "req:${REQ_FILE}" >>${FAIL_LIST}
echo "rsp:${RSP_FILE}" >>${FAIL_LIST}
echo "config:${TS_PATH}" >>${FAIL_LIST}
echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@" >>${FAIL_LIST}

# upload log to s3 and use cmd `aws s3 presign` to create a url for it

unset AWS_SECRET_KEY
unset AWS_ACCESS_KEY_ID

aws s3 cp --expires "$(date -d '+4 weeks' --utc +'%Y-%m-%dT%H:%M:%SZ')" --content-type "text/plain" ${LOG_PATH} $CASE_DIR/log/ || {
    echo "upload log file to aws failed"
    exit 23
}

url=$(aws s3 presign ${CASE_DIR}/log/$LOG_FILE --expires-in 1209600)

echo "gen fail-log for ${TS_PATH} done(expire in 3 days), url: <green>$url</green>"
