package binlog

import "testing"

const (
	mysqlPrimaryKeyDescription    = "PRI"            //主键
	mysqlAutoIncrementDescription = "auto_increment" //自增
)

func TestTransaction_MarshalJSON(t *testing.T) {
	testCases := []struct {
		input *Transaction
		want  string
	}{
		{
			input: &Transaction{
				NowPosition: testBinlogPosParseEvents,
				NextPosition: Position{
					Filename: testBinlogPosParseEvents.Filename,
					Offset:   4,
				},
				Events: []*StreamEvent{
					{
						Type:      StatementInsert,
						Timestamp: 1407805592,
						Table:     tesInfo.name,
						SQL:       "insert into vt_test_keyspace.vt_a(id,message)values(1076895760,'abcd')",
					},
					{
						Type:      StatementUpdate,
						Table:     tesInfo.name,
						Timestamp: 1407805592,
						RowIdentifies: []*RowData{
							{
								Columns: []*ColumnData{
									{
										Filed: "id",
										Data:  []byte("1076895760"),
										Type:  ColumnTypeLong,
									},
									{
										Filed: "message",
										Data:  []byte("abc"),
										Type:  ColumnTypeVarchar,
									},
								},
							},
						},
						RowValues: []*RowData{
							{
								Columns: []*ColumnData{
									{
										Filed: "id",
										Data:  []byte("1076895760"),
										Type:  ColumnTypeLong,
									},
									{
										Filed: "message",
										Data:  []byte("abcd"),
										Type:  ColumnTypeVarchar,
									},
								},
							},
						},
					},
					{
						Type:      StatementDelete,
						Timestamp: 1407805592,
						Table:     tesInfo.name,
						RowIdentifies: []*RowData{
							{
								Columns: []*ColumnData{
									{
										Filed: "id",
										Data:  []byte("1076895760"),
										Type:  ColumnTypeLong,
									},
									{
										Filed: "message",
										Data:  nil,
										Type:  ColumnTypeVarchar,
									},
								},
							},
						},
					},
				},
			},
			want: `{"nowPosition":{"filename":"binlog.000005","offset":0},"nextPosition":{"filename":"binlog.000005","offset":4},"timestamp":"1970-01-01 08:00:00 +0800 CST","events":[{"name":{"db":"vt_test_keyspace","table":"vt_a"},"type":"insert","timestamp":"2014-08-12 09:06:32 +0800 CST","sql":"insert into vt_test_keyspace.vt_a(id,message)values(1076895760,'abcd')"},{"name":{"db":"vt_test_keyspace","table":"vt_a"},"type":"update","timestamp":"2014-08-12 09:06:32 +0800 CST","rowValues":[{"Columns":[{"filed":"id","type":"Long","isEmpty":false,"data":"1076895760"},{"filed":"message","type":"Varchar","isEmpty":false,"data":"abcd"}]}],"rowIdentifies":[{"Columns":[{"filed":"id","type":"Long","isEmpty":false,"data":"1076895760"},{"filed":"message","type":"Varchar","isEmpty":false,"data":"abc"}]}]},{"name":{"db":"vt_test_keyspace","table":"vt_a"},"type":"delete","timestamp":"2014-08-12 09:06:32 +0800 CST","rowValues":null,"rowIdentifies":[{"Columns":[{"filed":"id","type":"Long","isEmpty":false,"data":"1076895760"},{"filed":"message","type":"Varchar","isEmpty":false,"data":null}]}]}]}`,
		},
	}
	for _, v := range testCases {
		out, err := v.input.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != v.want {
			//t.Log(string(out))
			t.Fatalf("want != out,want: %v,out: %v", v.want, string(out))
		}
	}
}
