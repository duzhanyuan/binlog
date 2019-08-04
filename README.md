# binlog

[![GoDoc][doc-img]][doc][![Build Status][ci-img]][ci][![Coverage Status][cov-img]][cov]

binlog将自己伪装成slave获取mysql主从复杂流来获取mysql数据库的数据变更，提供轻量级，快速的dump协议交互以及binlog的row模式下的格式解析

## Requests
+ mysql 5.6+
+ golang 1.9+

## Installation
go get github.com/onlyac0611/binlog

## Features
+ 轻量级，快速的dump协议交互以及binlog的row模式格式解析
+ 支持mysql 5.6.x以及5.7.x除了JSON，几何类型的所有数据类型变更
+ 支持使用完整dump协议连接数据库并接受binlog数据
+ 提供函数来接受解析后完整的事务数据
+ 事务数据提供变更的列名，列数据类型，bytes类型的数据

## Quick Start

### Prepare
+ 对于自建MySQL，需要先开启Binlog写入功能，配置binlog-format为ROW模式，使用config/my.cnf，配置如下:
```
[mysqld]
log-bin=mysql-bin # 开启 binlog
binlog-format=ROW # 选择 ROW 模式
server_id=1 # 配置 MySQL replaction 需要定义，不要和 canal 的 slaveId 重复
```
+ 授权examle链接MySQL账号具有作为MySQL slave的权限，如果已有账户可直接grant，使用scripts/grant.sql
```sql
CREATE USER example IDENTIFIED BY 'example';                                    #创建用户example
GRANT SELECT, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'example'@'%';    #授予SELECT,REPLICATION权限
FLUSH PRIVILEGES;                                                               #刷新权限
```
### Coding
+ 检查mysql的binlog格式是否是row模式，并且获取一个正确的binlog位置（以文件名和位移量作定义）
+ 实现MysqlTableMapper接口，该接口是用于获取表信息的，主要是获取列属性
+ 表meta.MysqlTable和列meta.MysqlColumn需要实现，用于MysqlTableMapper接口
+ 生成一个RowStreamer，设置一个正确的binlog位置并使用Stream接受数据，具体可以使用sendTransaction进行具体的行为定义

See the [binlogStream](tests/binlogStream/README.md) and [documentation][doc] for more details.

## Modular
### config
+ my.cnf 配置开启Binlog写入功能，配置binlog-format为ROW模式开启Binlog写入功能，配置binlog-format为ROW模式

### dump
这个模块是用于dump协议交互的，是从github.com/go-sql-driver/mysql的基础上移植过来
github.com/go-sql-driver/mysql已经支持了所有的协议包的读写，但是由于以下原因需要修改：
+ 该包不支持dump协议的交互
+ 该包在解析mysql数据时使用了缓存，导致与event冲突

目前dump已经支持单条连接完成dump协议的交互，取消了缓存机制，使其与event不再冲突

### replication
这个模块用于解析binlog的格式，是从github.com/youtube/vitess/go/mysql的基础上移植过来
github.com/youtube/vitess/go/mysql已经完整地支持mysql 5.6+所有的bonlog解析，但是由于以下原因需要修改：
+ 该包在vitess中有较多依赖，不便在其他项目中使用
+ 该包的mysql协议有些变化，如Decimal数据小数点后的缺少前置0等问题

目前event已经支持mysql 5.6.x以及5.7.x除了JSON，几何类型的所有数据类型变更，未来将支持全部

### scripts
+ conver.sh 用于计算项目的单元测试代码覆盖率
+ grant.sql 用于创建并授权example

[doc-img]: https://godoc.org/github.com/onlyac0611/binlog?status.svg
[doc]: https://godoc.org/github.com/onlyac0611/binlog
[ci-img]: https://travis-ci.com/onlyac0611/binlog.svg?branch=master
[ci]: https://travis-ci.com/onlyac0611/binlog
[cov-img]: https://codecov.io/gh/onlyac0611/binlog/branch/master/graph/badge.svg
[cov]: https://codecov.io/gh/onlyac0611/binlog