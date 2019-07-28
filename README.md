# binlog
这个项目是一个将自己伪装成slave获取mysql主从复杂流来获取mysql数据库的数据变更，提供轻量级，快速的dump协议交互以及binlog的row模式下的格式解析

## requests
+ mysql 5.6+
+ golang 1.9+

## Features
+ 轻量级，快速的dump协议交互以及binlog的row模式格式解析
+ 支持mysql 5.6.x以及5.7.x除了JSON，几何类型的所有数据类型变更
+ 支持使用完整dump协议连接数据库并接受binlog数据
+ 提供函数来接受解析后完整的事务数据
+ 事务数据提供变更的列名，列数据类型，bytes类型的数据

## Usage
### Steps
+ 1.检查mysql的binlog格式是否是row模式，并且获取一个正确的binlog位置（以文件名和位移量作定义）
+ 2.实现TableInfoMapper接口，该接口是用于获取表信息的，主要是获取列名和一些其他信息
+ 3.生成一个RowStreamer，设置一个正确的binlog位置并使用Stream接受数据，具体可以使用sendTransaction进行具体的行为定义

### Example
参考example_test.go,如果你期望调试dump协议交互以及binlog解析的过程，那么使用SetLogger函数将数据以你期望的方式打印调试信息

## Modular
### event
这个模块用于解析binlog的格式，是从github.com/youtube/vitess/go/mysql的基础上移植过来。
github.com/youtube/vitess/go/mysql已经完整地支持mysql 5.6+所有的bonlog解析，但是由于以下原因需要修改：
+ 该包在vitess中有较多依赖，不便在其他项目中使用
+ 该包的mysql协议有些变化，如Decimal数据小数点后的缺少前置0等问题

目前event已经支持mysql 5.6.x以及5.7.x除了JSON，几何类型的所有数据类型变更，未来将支持全部

### dump
这个模块是用于dump协议交互的，是从github.com/go-sql-driver/mysql的基础上移植过来。
github.com/go-sql-driver/mysql已经支持了所有的协议包的读写，但是由于以下原因需要修改：
+ 该包不支持dump协议的交互
+ 该包在解析mysql数据时使用了缓存，导致与event冲突

目前dump已经支持单条连接完成dump协议的交互，取消了缓存机制，使其与event不再冲突

### meta
这个模块是用于定义一些基本数据
+ binlog位置,以文件名和位移量作定义
+ 表信息，主要是表名和列信息
+ 事务信息，主要是一个完整的binlog events(以begin开始， 以commit结束)