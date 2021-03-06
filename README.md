# binlog

[![Go Report Card][report-img]][report]
[![Coverage Status][cov-img]][cov]
[![Build Status][ci-img]][ci]
[![GoDoc][doc-img]][doc]
[![LICENSE][license-img]][license]

binlog将自己伪装成slave获取mysql主从复杂流来获取mysql数据库的数据变更，提供轻量级，快速的dump协议交互以及binlog的row模式下的格式解析

## Features
+ 轻量级，快速的dump协议交互以及binlog的row模式格式解析
+ 支持mysql5.6以及mysql5.7除了JSON的所有数据类型变更
+ 支持使用完整dump协议连接数据库并接受binlog数据
+ 提供函数来接受解析后完整的事务数据
+ 事务数据提供变更的列名，列数据类型，bytes类型的数据

## Requests
+ mysql 5.6/mysql 5.7
+ golang 1.9+

## Installation
go get github.com/onlyac0611/binlog

## Quick Start
### Prepare
+ 对于自建MySQL，需要先开启Binlog写入功能，配置binlog-format为ROW模式
+ 授权examle链接MySQL账号具有作为MySQL slave的权限，如果已有账户可直接grant

### Coding
+ 检查mysql的binlog格式是否是row模式，并且获取一个正确的binlog位置（以文件名和位移量作定义）
+ 实现MysqlTableMapper接口，该接口是用于获取表信息的，主要是获取列属性
+ 表MysqlTable和列MysqlColumn需要实现，用于MysqlTableMapper接口
+ 生成一个RowStreamer，设置一个正确的binlog位置并使用Stream接受数据，具体可以使用sendTransaction进行具体的行为定义

See the [binlogStream](tests/binlogStream/README.md) and [documentation][doc] for more details.

[report-img]: https://goreportcard.com/badge/github.com/onlyac0611/binlog
[report]: https://goreportcard.com/report/github.com/onlyac0611/binlog
[cov-img]: https://codecov.io/gh/onlyac0611/binlog/branch/master/graph/badge.svg
[cov]: https://codecov.io/gh/onlyac0611/binlog
[ci-img]: https://travis-ci.com/onlyac0611/binlog.svg?branch=master
[ci]: https://travis-ci.com/onlyac0611/binlog
[doc-img]: https://godoc.org/github.com/onlyac0611/binlog?status.svg
[doc]: https://godoc.org/github.com/onlyac0611/binlog
[license-img]: https://img.shields.io/badge/License-Apache%202.0-blue.svg
[license]: https://github.com/onlyac0611/binlog/blob/master/LICENSE
