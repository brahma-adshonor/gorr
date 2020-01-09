#! /bin/bash

S3_FOLDER=${S3_CASE_DIR}
LOCAL_FOLDER=${LOCAL_CASE_DIR}

if [[ ${LOCAL_FOLDER} == "" || ${LOCAL_FOLDER} == "/" ]]; then
    echo "invalid local folder:${LOCAL_FOLDER}"
    exit 233
fi

for f in $(ls ${LOCAL_FOLDER}); do
    if [ -d $f ]; then
        echo "invalid local folder(${LOCAL_FOLDER}), subfolder detected:${f}"
        exit 234
    fi
done

CASE_FOLDER=$(basename ${LOCAL_FOLDER})

aws s3 cp ${LOCAL_FOLDER} ${S3_FOLDER}/${CASE_FOLDER} --recursive || {
    echo "upload to s3 failed:${LOCAL_FOLDER} -> ${S3_FOLDER}"
    exit 235
}

#rm -rf ${LOCAL_FOLDER}
echo "upload ${LOCAL_FOLDER} to s3 done"
