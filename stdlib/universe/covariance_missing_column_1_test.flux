package universe_test


import "testing"
import "csv"

inData =
    "
#datatype,string,long,dateTime:RFC3339,double,string
#group,false,false,false,false,true
#default,_result,,,,
,result,table,_time,x,_measurement
,,0,2018-05-22T19:53:26Z,0,cpu
,,0,2018-05-22T19:53:36Z,0,cpu
,,0,2018-05-22T19:53:46Z,2,cpu
,,0,2018-05-22T19:53:56Z,7,cpu
"

testcase covariance_missing_column_1 {
    testing.shouldError(
        fn: () =>
            csv.from(csv: inData)
                |> covariance(columns: ["x", "r"])
                |> tableFind(fn: (key) => true),
        want:
            "error calling function \"tableFind\" @stdlib/universe/covariance_missing_column_1_test.flux|24:20-24:48: runtime error @stdlib/universe/covariance_missing_column_1_test.flux|23:20-23:51: covariance: specified column does not exist in table: r",
    )
}
