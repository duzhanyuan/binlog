# binlogStream

binlogStream将自己伪装成slave获取mysql主从复杂流来获取mysql数据库的数据变更,以json格式输出，是github.com/onlyac0611/binlog测试的一个例子

## Requests
+ mysql 5.6+
+ golang 1.9+

## Installation
go get github.com/onlyac0611/binlog
go get github.com/go-sql-driver/mysql
make example

## Quick Start
### Prepare
+ 对于自建MySQL，需要先开启Binlog写入功能，配置binlog-format为ROW模式，my.cnf中配置如下:
```
[mysqld]
log-bin=mysql-bin # 开启 binlog
binlog-format=ROW # 选择 ROW 模式
server_id=1 # 配置 MySQL replaction 需要定义，不要和 canal 的 slaveId 重复
```
+ 授权examle链接MySQL账号具有作为MySQL slave的权限，如果已有账户可直接grant
```sql
CREATE USER example IDENTIFIED BY 'example';                                    #创建用户example
GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'example'@'%';    #授予SELECT,REPLICATION权限
FLUSH PRIVILEGES;                                                               #刷新权限
```

### Run
使用./binlogStream
测试脚本为tests/type_test.sql

### Result
参考transaction.txt的输出
如对于插入语句
```sql
INSERT INTO type_table (
    t_tinyint, t_smallint, t_mediumint, t_int, t_bigint, t_bit,
    t_float, t_double, t_decimal,
    t_char, t_varchar, t_tinytext, t_text, t_mediumtext, t_longtext,
    t_binary, t_varbinary, t_tinyblob, t_blob, t_mediumblob, t_longblob,
    t_date, t_time, t_datetime, t_timestamp,
    `t_中文列`,`t_unsigned_bigint`
) VALUES (
    1, 1, 1, 1, 0, b'0', 0, 0, 0.0000,
    'char', uuid(), 'tinytext', 'text', 'mediumtext', 'longtext',
     _binary 0x1234567890ABCDEF000000000000000000000000000000000000000000000000000000000000000000000000000000000000,
     _binary 0x1234567890ABCDEF,
     _binary 0x1234567890ABCDEF,
     _binary 0x1234567890ABCDEF,
     _binary 0x1234567890ABCDEF,
     _binary 0x121345673FBCDE,
     '1900-01-01', '26:00:00', now(), '2016-11-21 14:51:53',
     '中文列',18446744073709551615
);
```

对应的json结构体如下：
```json
{
  "nowPosition":{
    "filename":"mysql-bin.000003",
    "offset":49318
  },
  "nextPosition":{
    "filename":"mysql-bin.000003",
    "offset":51540
  },
  "timestamp":"2019-08-04 19:02:48 +0800 CST",
  "events":[
    {
      "name":{
        "db":"test",
        "table":"type_table"
      },
      "type":"insert",
      "timestamp":"2019-08-04 19:02:48 +0800 CST",
      "rowValues":[
        {
          "Columns":[
            {
              "filed":"t_primary",
              "type":"Long",
              "isEmpty":false,
              "data":"1"
            },
            {
              "filed":"t_bit",
              "type":"Bit",
              "isEmpty":false,
              "data":"\u0000"
            },
            {
              "filed":"t_tinyint",
              "type":"Tiny",
              "isEmpty":false,
              "data":"1"
            },
            {
              "filed":"t_smallint",
              "type":"Short",
              "isEmpty":false,
              "data":"1"
            },
            {
              "filed":"t_mediumint",
              "type":"Int24",
              "isEmpty":false,
              "data":"1"
            },
            {
              "filed":"t_int",
              "type":"Long",
              "isEmpty":false,
              "data":"1"
            },
            {
              "filed":"t_bigint",
              "type":"LongLong",
              "isEmpty":false,
              "data":"0"
            },
            {
              "filed":"t_float",
              "type":"Float",
              "isEmpty":false,
              "data":"0"
            },
            {
              "filed":"t_double",
              "type":"Double",
              "isEmpty":false,
              "data":"0"
            },
            {
              "filed":"t_decimal",
              "type":"NewDecimal",
              "isEmpty":false,
              "data":"0.00"
            },
            {
              "filed":"t_char",
              "type":"String",
              "isEmpty":false,
              "data":"char"
            },
            {
              "filed":"t_varchar",
              "type":"Varchar",
              "isEmpty":false,
              "data":"6532f239-b6a7-11e9-9a38-409f38b73ce5"
            },
            {
              "filed":"t_tinytext",
              "type":"Blob",
              "isEmpty":false,
              "data":"tinytext"
            },
            {
              "filed":"t_text",
              "type":"Blob",
              "isEmpty":false,
              "data":"text"
            },
            {
              "filed":"t_mediumtext",
              "type":"Blob",
              "isEmpty":false,
              "data":"mediumtext"
            },
            {
              "filed":"t_longtext",
              "type":"Blob",
              "isEmpty":false,
              "data":"longtext"
            },
            {
              "filed":"t_binary",
              "type":"String",
              "isEmpty":false,
              "data":"\u00124Vx����"
            },
            {
              "filed":"t_varbinary",
              "type":"Varchar",
              "isEmpty":false,
              "data":"\u00124Vx����"
            },
            {
              "filed":"t_tinyblob",
              "type":"Blob",
              "isEmpty":false,
              "data":"\u00124Vx����"
            },
            {
              "filed":"t_blob",
              "type":"Blob",
              "isEmpty":false,
              "data":"\u00124Vx����"
            },
            {
              "filed":"t_mediumblob",
              "type":"Blob",
              "isEmpty":false,
              "data":"\u00124Vx����"
            },
            {
              "filed":"t_longblob",
              "type":"Blob",
              "isEmpty":false,
              "data":"\u0012\u0013Eg?��"
            },
            {
              "filed":"t_date",
              "type":"Date",
              "isEmpty":false,
              "data":"1900-01-01"
            },
            {
              "filed":"t_time",
              "type":"Time2",
              "isEmpty":false,
              "data":"26:00:00"
            },
            {
              "filed":"t_datetime",
              "type":"DateTime2",
              "isEmpty":false,
              "data":"2019-08-04 19:02:48"
            },
            {
              "filed":"t_timestamp",
              "type":"Timestamp2",
              "isEmpty":false,
              "data":"2016-11-21 14:51:53"
            },
            {
              "filed":"t_中文列",
              "type":"Varchar",
              "isEmpty":false,
              "data":"中文列"
            },
            {
              "filed":"t_unsigned_bigint",
              "type":"LongLong",
              "isEmpty":false,
              "data":"18446744073709551615"
            }
          ]
        }
      ],
      "rowIdentifies":[
        
      ]
    }
  ]
}
```