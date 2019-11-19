# /bin/bash

if [[ "${INSTALL_TYPE}" != "REGRESSION" ]]; then
    exit 20
fi

mkdir -p ${INSTALL_DIR}/tmp || true
mkdir -p ${INSTALL_DIR}/regression || true

if $(command -v aws >/dev/null 2>&1); then
    echo "try to get test cases from $CASE_DIR"
    aws s3 cp $CASE_DIR ${INSTALL_DIR}/regression --recursive >/dev/null 2>&1
    if [[ "$?" != 0 ]]; then
        echo "get test case from $CASE_DIR failed"
        exit 30
    else
        echo "done getting test case from $CASE_DIR"
    fi
fi

chmod 755 ${REGRESSION_RUNNER}
echo "starting regression_runner @ ${INSTALL_DIR}..."

echo >${INSTALL_DIR}/regression/fail.again || true
echo >${INSTALL_DIR}/regression/file.changed || true

on_fail_handler="cat ${SERVER_LOG} >> ${INSTALL_DIR}/regression/run.log"

update_case_on_fail=0
if [[ ${UPDATE_FAIL_CASE} == "1" ]]; then
    update_case_on_fail=1
fi

${REGRESSION_RUNNER} \
    -server_stop_cmd="${SERVER_STOP}" \
    -server_start_cmd="${SERVER_START}" \
    -runner="${REQ_CLIENT}" \
    -server_addr=${SERVER_ADDR} \
    -test_case_dir=${INSTALL_DIR}/regression \
    -tmp_store_dir=${INSTALL_DIR}/regression/tmp \
    -regression_db_path=${REGRESSION_DB_DIR} \
    -update_case_from_diff=${update_case_on_fail} \
    -on_test_suit_fail_handler="${on_fail_handler}" \
    -regression_flag=${REGRESSION_FLAG_FILE} \
    -output_fail_again=${INSTALL_DIR}/regression/fail.again \
    -output_file_changed=${INSTALL_DIR}/regression/file.changed >${INSTALL_DIR}/regression/run.log 2>&1

ec=$?
/bin/bash ${SERVER_STOP} || true

if [[ "$ec" != "0" && ${ERR_FORWARDER} != "" ]]; then
    msg=$(cat ${INSTALL_DIR}/regression/run.log)
    curl "$ERR_FORWARDER" -d "cirobot-msg-forwarder:\n${msg}" || true
fi

fa=$(cat ${INSTALL_DIR}/regression/fail.again)
if [[ "${fa}" != "" ]]; then
    curl "$ERR_FORWARDER" -d "cirobot-msg-forwarder:\n${fa}" || true
fi

cd ${INSTALL_DIR}/regression
cat ${INSTALL_DIR}/regression/run.log

prefix=${INSTALL_DIR}/regression/
for f in $(cat ${INSTALL_DIR}/regression/file.changed); do
    target=${f#$prefix}
    aws s3 cp $f $CASE_DIR/$target || {
        echo "upload file change failed:$f"
        continue
    }
    echo "upload file succeeded:$f"
done

exit $ec
