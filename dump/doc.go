/*
Package dump 用于dump协议交互的，
从github.com/go-sql-driver/mysql的基础上修改而来，主要功能如下：
1.通过MysqlConn可以执行简单的sql命令，如set命令，
2.通过MysqlConn来和mysql库进行binlog dump

github.com/go-sql-driver/mysql已经支持了所有的协议包的读写，但是
由于以下原因需要修改：1.该包不支持dump协议的交互。2.在解析mysql网络
数据重复使用了同一个内存块，由于Package replication中每一个binlogEvent
需要独立缓存一个内存块，导致binlogEvent会读取脏数据。

目前dump已经支持单条连接完成dump协议的交互，取消了重复使用内存机制，
使其在mysql网络数据时使用独立缓存，从而保障了与Package replication
不再读到脏数据。

DSN参数说明：

[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
username是mysql的用户名，password是mysql的密码，protocol协议一般是tcp,address是mysql的地址，
dbname是mysql的数据库名

常用参数说明：

charset

Type:           string
Valid Values:   <name>
Default:        none
Sets the charset used for client-server interaction ("SET NAMES <value>"). If multiple charsets are set (separated by a comma), the following charset is used if setting the charset failes. This enables for example support for utf8mb4 (introduced in MySQL 5.5.3) with fallback to utf8 for older servers (charset=utf8mb4,utf8).

Usage of the charset parameter is discouraged because it issues additional queries to the server. Unless you need the fallback behavior, please use collation instead.

collation

Type:           string
Valid Values:   <name>
Default:        utf8mb4_general_ci
Sets the collation used for client-server interaction on connection. In contrast to charset, collation does not issue additional queries. If the specified collation is unavailable on the target server, the connection will fail.

A list of valid charsets for a server is retrievable with SHOW COLLATION.

The default collation (utf8mb4_general_ci) is supported from MySQL 5.5. You should use an older collation (e.g. utf8_general_ci) for older MySQL.

Collations for charset "ucs2", "utf16", "utf16le", and "utf32" can not be used (ref).

loc

Type:           string
Valid Values:   <escaped name>
Default:        UTC
Sets the location for time.Time values (when using parseTime=true). "Local" sets the system's location. See time.LoadLocation for details.

Note that this sets the location for time.Time values but does not change MySQL's time_zone setting. For that see the time_zone system variable, which can also be set as a DSN parameter.

Please keep in mind, that param values must be url.QueryEscape'ed. Alternatively you can manually replace the / with %2F. For example US/Pacific would be loc=US%2FPacific.

maxAllowedPacket

Type:          decimal number
Default:       4194304
Max packet size allowed in bytes. The default value is 4 MiB and should be adjusted to match the server settings. maxAllowedPacket=0 can be used to automatically fetch the max_allowed_packet variable from server on every connection.

parseTime

Type:           bool
Valid Values:   true, false
Default:        false
parseTime=true changes the output type of DATE and DATETIME values to time.Time instead of []byte / string The date or datetime like 0000-00-00 00:00:00 is converted into zero value of time.Time.

readTimeout

Type:           duration
Default:        0
I/O read timeout. The value must be a decimal number with a unit suffix ("ms", "s", "m", "h"), such as "30s", "0.5m" or "1m30s".

timeout

Type:           duration
Default:        OS default
Timeout for establishing connections, aka dial timeout. The value must be a decimal number with a unit suffix ("ms", "s", "m", "h"), such as "30s", "0.5m" or "1m30s".

writeTimeout

Type:           duration
Default:        0
I/O write timeout. The value must be a decimal number with a unit suffix ("ms", "s", "m", "h"), such as "30s", "0.5m" or "1m30s".
*/
package dump
