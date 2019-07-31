// package dump 用于dump协议交互的，从github.com/go-sql-driver/mysql的基础上修改而来，主要功能如下：
// 通过MysqlConn可以执行简单的sql命令，如set命令，
// 通过MysqlConn来和mysql库进行binlog dump
//
// github.com/go-sql-driver/mysql已经支持了所有的协议包的读写，但是由于以下原因需要修改：
// 该包不支持dump协议的交互，
// 该包在解析mysql数据时使用了缓存，导致与event冲突。
//
// 目前dump已经支持单条连接完成dump协议的交互，取消了缓存机制，使其与event不再冲突。
package dump

import (
	"bufio"
	"net"
	"strings"
	"time"
)

const defaultBufSize = 4096

//mysql连接，用于执行dump和其他命令
type MysqlConn struct {
	//buf              buffer
	reader           *bufio.Reader
	netConn          net.Conn
	affectedRows     uint64
	insertId         uint64
	cfg              *Config
	maxAllowedPacket int
	maxWriteSize     int
	writeTimeout     time.Duration
	flags            clientFlag
	status           statusFlag
	sequence         uint8
	parseTime        bool
	strict           bool
}

func NewMysqlConn(dsn string) (*MysqlConn, error) {
	var err error

	// New MysqlConn
	mc := &MysqlConn{
		maxAllowedPacket: maxPacketSize,
		maxWriteSize:     maxPacketSize - 1,
	}
	mc.cfg, err = ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	mc.parseTime = mc.cfg.ParseTime
	mc.strict = mc.cfg.Strict

	nd := net.Dialer{Timeout: mc.cfg.Timeout}
	mc.netConn, err = nd.Dial(mc.cfg.Net, mc.cfg.Addr)

	if err != nil {
		return nil, err
	}

	// Enable TCP Keepalives on TCP connections
	if tc, ok := mc.netConn.(*net.TCPConn); ok {
		if err := tc.SetKeepAlive(true); err != nil {
			// Don't send COM_QUIT before handshake.
			mc.netConn.Close()
			mc.netConn = nil
			return nil, err
		}
	}
	mc.reader = bufio.NewReaderSize(mc.netConn, defaultBufSize)
	//mc.buf = newBuffer(mc.netConn)

	// Set I/O timeouts
	//mc.buf.timeout = mc.cfg.ReadTimeout
	mc.writeTimeout = mc.cfg.WriteTimeout

	// Reading Handshake Initialization Packet
	cipher, err := mc.readInitPacket()
	if err != nil {
		mc.cleanup()
		return nil, err
	}

	// Send Client Authentication Packet
	if err = mc.writeAuthPacket(cipher); err != nil {
		mc.cleanup()
		return nil, err
	}

	// Handle response to auth packet, switch methods if possible
	if err = handleAuthResult(mc, cipher); err != nil {
		// Authentication failed and MySQL has already closed the connection
		// (https://dev.mysql.com/doc/internals/en/authentication-fails.html).
		// Do not send COM_QUIT, just cleanup and return the error.
		mc.cleanup()
		return nil, err
	}

	if mc.cfg.MaxAllowedPacket > 0 {
		mc.maxAllowedPacket = mc.cfg.MaxAllowedPacket
	} else {
		// Get max allowed packet size
		maxap, err := mc.getSystemVar("max_allowed_packet")
		if err != nil {
			mc.Close()
			return nil, err
		}
		mc.maxAllowedPacket = stringToInt(maxap) - 1
	}
	if mc.maxAllowedPacket < maxPacketSize {
		mc.maxWriteSize = mc.maxAllowedPacket
	}

	// Handle DSN Params
	err = mc.handleParams()
	if err != nil {
		mc.Close()
		return nil, err
	}

	return mc, nil
}

// Handles parameters set in DSN after the connection is established
func (mc *MysqlConn) handleParams() (err error) {
	for param, val := range mc.cfg.Params {
		switch param {
		// Charset
		case "charset":
			charsets := strings.Split(val, ",")
			for i := range charsets {
				// ignore errors here - a charset may not exist
				err = mc.exec("SET NAMES " + charsets[i])
				if err == nil {
					break
				}
			}
			if err != nil {
				return
			}

		// System Vars
		default:
			err = mc.exec("SET " + param + "=" + val + "")
			if err != nil {
				return
			}
		}
	}

	return
}

func (mc *MysqlConn) Close() (err error) {
	// Makes Close idempotent
	if mc.netConn != nil {
		err = mc.writeCommandPacket(comQuit)
	}

	mc.cleanup()

	return
}

// Gets the value of the given MySQL System Variable
// The returned byte slice is only valid until the next read
func (mc *MysqlConn) getSystemVar(name string) ([]byte, error) {
	// Send command
	if err := mc.writeCommandPacketStr(comQuery, "SELECT @@"+name); err != nil {
		return nil, err
	}

	// Read Result
	resLen, err := mc.readResultSetHeaderPacket()
	if err == nil {
		rows := new(textRows)
		rows.mc = mc
		rows.columns = []mysqlField{{fieldType: fieldTypeVarChar}}

		if resLen > 0 {
			// Columns
			if err := mc.readUntilEOF(); err != nil {
				return nil, err
			}
		}

		dest := make([]interface{}, resLen)
		if err = rows.readRow(dest); err == nil {
			return dest[0].([]byte), mc.readUntilEOF()
		}
	}
	return nil, err
}

// Closes the network connection and unsets internal variables. Do not call this
// function after successfully authentication, call Close instead. This function
// is called before auth or on auth failure because MySQL will have already
// closed the network connection.
func (mc *MysqlConn) cleanup() {
	// Makes cleanup idempotent
	if mc.netConn != nil {
		if err := mc.netConn.Close(); err != nil {
			errLog.Print(err)
		}
		mc.netConn = nil
	}
	mc.cfg = nil
}

// Internal function to execute commands
func (mc *MysqlConn) Exec(query string) error {
	return mc.exec(query)
}

//通知开始从哪个binlog位置开始以serverID为编号开始同步数据
func (mc *MysqlConn) NoticeDump(serverID, offset uint32, filename string, flags uint16) error {
	return mc.writeDumpBinlogPosPacket(serverID, offset, filename, flags)
}

//读取mysql协议包
func (mc *MysqlConn) ReadPacket() ([]byte, error) {
	return mc.readPacket()
}

//处理mysql返回的错误
func (mc *MysqlConn) HandleErrorPacket(data []byte) error {
	return mc.handleErrorPacket(data)
}

func (mc *MysqlConn) exec(query string) error {
	// Send command
	err := mc.writeCommandPacketStr(comQuery, query)
	if err != nil {
		return err
	}

	// Read Result
	resLen, err := mc.readResultSetHeaderPacket()
	if err == nil && resLen > 0 {
		if err = mc.readUntilEOF(); err != nil {
			return err
		}

		err = mc.readUntilEOF()
	}

	return err
}

func (mc *MysqlConn) query(query string) (MyRows, error) {
	err := mc.writeCommandPacketStr(comQuery, query)
	if err == nil {
		// Read Result
		var resLen int
		resLen, err = mc.readResultSetHeaderPacket()
		if err == nil {
			rows := new(textRows)
			rows.mc = mc

			if resLen == 0 {
				// no columns, no more data
				return emptyRows{}, nil
			}
			// Columns
			rows.columns, err = mc.readColumns(resLen)
			return rows, err
		}
	}
	return nil, err
}

func handleAuthResult(mc *MysqlConn, oldCipher []byte) error {
	// Read Result Packet
	cipher, err := mc.readResultOK()
	if err == nil {
		return nil // auth successful
	}

	if mc.cfg == nil {
		return err // auth failed and retry not possible
	}

	// Retry auth if configured to do so.
	if mc.cfg.AllowOldPasswords && err == ErrOldPassword {
		// Retry with old authentication method. Note: there are edge cases
		// where this should work but doesn't; this is currently "wontfix":
		// https://github.com/go-sql-driver/mysql/issues/184

		// If CLIENT_PLUGIN_AUTH capability is not supported, no new cipher is
		// sent and we have to keep using the cipher sent in the init packet.
		if cipher == nil {
			cipher = oldCipher
		}

		if err = mc.writeOldAuthPacket(cipher); err != nil {
			return err
		}
		_, err = mc.readResultOK()
	} else if mc.cfg.AllowCleartextPasswords && err == ErrCleartextPassword {
		// Retry with clear text password for
		// http://dev.mysql.com/doc/refman/5.7/en/cleartext-authentication-plugin.html
		// http://dev.mysql.com/doc/refman/5.7/en/pam-authentication-plugin.html
		if err = mc.writeClearAuthPacket(); err != nil {
			return err
		}
		_, err = mc.readResultOK()
	} else if mc.cfg.AllowNativePasswords && err == ErrNativePassword {
		if err = mc.writeNativeAuthPacket(cipher); err != nil {
			return err
		}
		_, err = mc.readResultOK()
	}
	return err
}
