package dump

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"
)

func (mc *MysqlConn) setReadTimeout() error {
	if mc.readTimeout > 0 {
		return mc.netConn.SetReadDeadline(time.Now().Add(mc.readTimeout))
	}
	return nil
}

func (mc *MysqlConn) cleanReadTimeout() error {
	if mc.readTimeout > 0 {
		return mc.netConn.SetReadDeadline(time.Time{})
	}
	return nil
}

// Packets documentation:
// http://dev.mysql.com/doc/internals/en/client-server-protocol.html

// Read packet to buffer 'data'
func (mc *MysqlConn) readPacket() ([]byte, error) {
	var prevData []byte
	var header [4]byte
	for {
		// read packet header
		if err := mc.setReadTimeout(); err != nil {
			return nil, err
		}

		if _, err := io.ReadFull(mc.reader, header[:]); err != nil {
			errLog.Print(fmt.Errorf("io.ReadFull(header size) failed: %v", err))
			mc.Close()
			return nil, ErrBadConn
		}

		if err := mc.cleanReadTimeout(); err != nil {
			return nil, err
		}
		// packet length [24 bit]
		pktLen := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)

		// check packet sync [8 bit]
		if header[3] != mc.sequence {
			if header[3] > mc.sequence {
				return nil, ErrPktSyncMul
			}
			return nil, ErrPktSync
		}
		mc.sequence++

		// packets with length 0 terminate a previous packet which is a
		// multiple of (2^24)−1 bytes long
		if pktLen == 0 {
			// there was no previous packet
			if prevData == nil {
				errLog.Print(ErrMalformPkt)
				mc.Close()
				return nil, ErrBadConn
			}

			return prevData, nil
		}

		// read packet body [pktLen bytes]
		if err := mc.setReadTimeout(); err != nil {
			return nil, err
		}

		data := make([]byte, pktLen)
		if _, err := io.ReadFull(mc.reader, data); err != nil {
			errLog.Print("io.ReadFull(packet body of length %v) failed: %v", pktLen, err)
			mc.Close()
			return nil, ErrBadConn
		}

		if err := mc.cleanReadTimeout(); err != nil {
			return nil, err
		}
		// return data if this was the last packet
		if pktLen < maxPacketSize {
			// zero allocations for non-split packets
			if prevData == nil {
				return data, nil
			}

			return append(prevData, data...), nil
		}

		prevData = append(prevData, data...)
	}
}

// Write packet buffer 'data'
func (mc *MysqlConn) writePacket(data []byte) error {
	pktLen := len(data) - 4

	if pktLen > mc.maxAllowedPacket {
		return ErrPktTooLarge
	}

	for {
		var size int
		if pktLen >= maxPacketSize {
			data[0] = 0xff
			data[1] = 0xff
			data[2] = 0xff
			size = maxPacketSize
		} else {
			data[0] = byte(pktLen)
			data[1] = byte(pktLen >> 8)
			data[2] = byte(pktLen >> 16)
			size = pktLen
		}
		data[3] = mc.sequence

		// Write packet
		if mc.writeTimeout > 0 {
			if err := mc.netConn.SetWriteDeadline(time.Now().Add(mc.writeTimeout)); err != nil {
				return err
			}
		}

		n, err := mc.netConn.Write(data[:4+size])
		if err == nil && n == 4+size {
			mc.sequence++
			if size != maxPacketSize {
				return nil
			}
			pktLen -= size
			data = data[size:]
			continue
		}

		// Handle error
		if err == nil { // n != len(data)
			errLog.Print(ErrMalformPkt)
		} else {
			errLog.Print(err)
		}
		return ErrBadConn
	}
}

/******************************************************************************
*                           Initialisation Process                            *
******************************************************************************/

// Handshake Initialization Packet
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::Handshake
func (mc *MysqlConn) readInitPacket() ([]byte, error) {
	data, err := mc.readPacket()
	if err != nil {
		return nil, err
	}

	if data[0] == iERR {
		return nil, mc.handleErrorPacket(data)
	}

	// protocol version [1 byte]
	if data[0] < minProtocolVersion {
		return nil, fmt.Errorf(
			"unsupported protocol version %d. Version %d or higher is required",
			data[0],
			minProtocolVersion,
		)
	}

	// server version [null terminated string]
	// connection id [4 bytes]
	pos := 1 + bytes.IndexByte(data[1:], 0x00) + 1 + 4

	// first part of the password cipher [8 bytes]
	cipher := data[pos : pos+8]

	// (filler) always 0x00 [1 byte]
	pos += 8 + 1

	// capability flags (lower 2 bytes) [2 bytes]
	mc.flags = clientFlag(binary.LittleEndian.Uint16(data[pos : pos+2]))
	if mc.flags&clientProtocol41 == 0 {
		return nil, ErrOldProtocol
	}
	if mc.flags&clientSSL == 0 && mc.cfg.tls != nil {
		return nil, ErrNoTLS
	}
	pos += 2

	if len(data) > pos {
		// character set [1 byte]
		// status flags [2 bytes]
		// capability flags (upper 2 bytes) [2 bytes]
		// length of auth-plugin-data [1 byte]
		// reserved (all [00]) [10 bytes]
		pos += 1 + 2 + 2 + 1 + 10

		// second part of the password cipher [mininum 13 bytes],
		// where len=MAX(13, length of auth-plugin-data - 8)
		//
		// The web documentation is ambiguous about the length. However,
		// according to mysql-5.7/sql/auth/sql_authentication.cc line 538,
		// the 13th byte is "\0 byte, terminating the second part of
		// a scramble". So the second part of the password cipher is
		// a NULL terminated string that's at least 13 bytes with the
		// last byte being NULL.
		//
		// The official Python library uses the fixed length 12
		// which seems to work but technically could have a hidden bug.
		cipher = append(cipher, data[pos:pos+12]...)

		// TODO: Verify string termination
		// EOF if version (>= 5.5.7 and < 5.5.10) or (>= 5.6.0 and < 5.6.2)
		// \NUL otherwise
		//
		//if data[len(data)-1] == 0 {
		//	return
		//}
		//return ErrMalformPkt

		// make a memory safe copy of the cipher slice
		var b [20]byte
		copy(b[:], cipher)
		return b[:], nil
	}

	// make a memory safe copy of the cipher slice
	var b [8]byte
	copy(b[:], cipher)
	return b[:], nil
}

// Client Authentication Packet
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::HandshakeResponse
func (mc *MysqlConn) writeAuthPacket(cipher []byte) error {
	// Adjust client flags based on server support
	clientFlags := clientProtocol41 |
		clientSecureConn |
		clientLongPassword |
		clientTransactions |
		clientLocalFiles |
		clientPluginAuth |
		clientMultiResults |
		mc.flags&clientLongFlag

	if mc.cfg.ClientFoundRows {
		clientFlags |= clientFoundRows
	}

	// To enable TLS / SSL
	if mc.cfg.tls != nil {
		clientFlags |= clientSSL
	}

	if mc.cfg.MultiStatements {
		clientFlags |= clientMultiStatements
	}

	// User Password
	scrambleBuff := scramblePassword(cipher, []byte(mc.cfg.Passwd))

	pktLen := 4 + 4 + 1 + 23 + len(mc.cfg.User) + 1 + 1 + len(scrambleBuff) + 21 + 1

	// To specify a db name
	if n := len(mc.cfg.DBName); n > 0 {
		clientFlags |= clientConnectWithDB
		pktLen += n + 1
	}

	// Calculate packet length and get buffer with that size
	data := make([]byte, pktLen+4)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// ClientFlags [32 bit]
	data[4] = byte(clientFlags)
	data[5] = byte(clientFlags >> 8)
	data[6] = byte(clientFlags >> 16)
	data[7] = byte(clientFlags >> 24)

	// MaxPacketSize [32 bit] (none)
	data[8] = 0x00
	data[9] = 0x00
	data[10] = 0x00
	data[11] = 0x00

	// Charset [1 byte]
	var found bool
	data[12], found = collations[mc.cfg.Collation]
	if !found {
		// Note possibility for false negatives:
		// could be triggered  although the collation is valid if the
		// collations map does not contain entries the server supports.
		return errors.New("unknown collation")
	}

	// SSL Connection Request Packet
	// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::SSLRequest
	if mc.cfg.tls != nil {
		// Send TLS / SSL request packet
		if err := mc.writePacket(data[:(4+4+1+23)+4]); err != nil {
			return err
		}

		// Switch to TLS
		tlsConn := tls.Client(mc.netConn, mc.cfg.tls)
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		mc.netConn = tlsConn
	}

	// Filler [23 bytes] (all 0x00)
	pos := 13
	for ; pos < 13+23; pos++ {
		data[pos] = 0
	}

	// User [null terminated string]
	if len(mc.cfg.User) > 0 {
		pos += copy(data[pos:], mc.cfg.User)
	}
	data[pos] = 0x00
	pos++

	// ScrambleBuffer [length encoded integer]
	data[pos] = byte(len(scrambleBuff))
	pos += 1 + copy(data[pos+1:], scrambleBuff)

	// Databasename [null terminated string]
	if len(mc.cfg.DBName) > 0 {
		pos += copy(data[pos:], mc.cfg.DBName)
		data[pos] = 0x00
		pos++
	}

	// Assume native client during response
	pos += copy(data[pos:], "mysql_native_password")
	data[pos] = 0x00

	// Send Auth packet
	return mc.writePacket(data)
}

//  Client old authentication packet
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (mc *MysqlConn) writeOldAuthPacket(cipher []byte) error {
	// User password
	scrambleBuff := scrambleOldPassword(cipher, []byte(mc.cfg.Passwd))

	// Calculate the packet length and add a tailing 0
	pktLen := len(scrambleBuff) + 1
	data := make([]byte, 4+pktLen)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// Add the scrambled password [null terminated string]
	copy(data[4:], scrambleBuff)
	data[4+pktLen-1] = 0x00

	return mc.writePacket(data)
}

//  Client clear text authentication packet
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (mc *MysqlConn) writeClearAuthPacket() error {
	// Calculate the packet length and add a tailing 0
	pktLen := len(mc.cfg.Passwd) + 1
	data := make([]byte, 4+pktLen)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// Add the clear password [null terminated string]
	copy(data[4:], mc.cfg.Passwd)
	data[4+pktLen-1] = 0x00

	return mc.writePacket(data)
}

//  Native password authentication method
// http://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::AuthSwitchResponse
func (mc *MysqlConn) writeNativeAuthPacket(cipher []byte) error {
	scrambleBuff := scramblePassword(cipher, []byte(mc.cfg.Passwd))

	// Calculate the packet length and add a tailing 0
	pktLen := len(scrambleBuff)
	data := make([]byte, 4+pktLen)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// Add the scramble
	copy(data[4:], scrambleBuff)

	return mc.writePacket(data)
}

/******************************************************************************
*                             Command Packets                                 *
******************************************************************************/

func (mc *MysqlConn) writeCommandPacketStr(command byte, arg string) error {
	// Reset Packet Sequence
	mc.sequence = 0

	pktLen := 1 + len(arg)
	data := make([]byte, pktLen+4)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// Add command byte
	data[4] = command

	// Add arg
	copy(data[5:], arg)

	// Send CMD packet
	return mc.writePacket(data)
}

func (mc *MysqlConn) writeCommandPacket(command byte) error {
	// Reset Packet Sequence
	mc.sequence = 0

	data := make([]byte, 4+1)
	if data == nil {
		// can not take the buffer. Something must be wrong with the connection
		errLog.Print(ErrBusyBuffer)
		return ErrBadConn
	}

	// Add command byte
	data[4] = command

	// Send CMD packet
	return mc.writePacket(data)
}

/******************************************************************************
*                              Result Packets                                 *
******************************************************************************/

// Returns error if Packet is not an 'Result OK'-Packet
func (mc *MysqlConn) readResultOK() ([]byte, error) {
	data, err := mc.readPacket()
	if err == nil {
		// packet indicator
		switch data[0] {

		case iOK:
			return nil, mc.handleOkPacket(data)

		case iEOF:
			if len(data) > 1 {
				pluginEndIndex := bytes.IndexByte(data, 0x00)
				plugin := string(data[1:pluginEndIndex])
				cipher := data[pluginEndIndex+1 : len(data)-1]

				if plugin == "mysql_old_password" {
					// using old_passwords
					return cipher, ErrOldPassword
				} else if plugin == "mysql_clear_password" {
					// using clear text password
					return cipher, ErrCleartextPassword
				} else if plugin == "mysql_native_password" {
					// using mysql default authentication method
					return cipher, ErrNativePassword
				} else {
					return cipher, ErrUnknownPlugin
				}
			} else {
				// https://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::OldAuthSwitchRequest
				return nil, ErrOldPassword
			}

		default: // Error otherwise
			return nil, mc.handleErrorPacket(data)
		}
	}
	return nil, err
}

// Result Set Header Packet
// http://dev.mysql.com/doc/internals/en/com-query-response.html#packet-ProtocolText::Resultset
func (mc *MysqlConn) readResultSetHeaderPacket() (int, error) {
	data, err := mc.readPacket()
	if err == nil {
		switch data[0] {

		case iOK:
			return 0, mc.handleOkPacket(data)

		case iERR:
			return 0, mc.handleErrorPacket(data)

			/*case iLocalInFile:
			return 0, mc.handleInFileRequest(string(data[1:]))*/
		}

		// column count
		num, _, n := readLengthEncodedInteger(data)
		if n-len(data) == 0 {
			return int(num), nil
		}

		return 0, ErrMalformPkt
	}
	return 0, err
}

// Error Packet
// http://dev.mysql.com/doc/internals/en/generic-response-packets.html#packet-ERR_Packet
func (mc *MysqlConn) handleErrorPacket(data []byte) error {
	if data[0] != iERR {
		return ErrMalformPkt
	}

	// 0xff [1 byte]

	// Error Number [16 bit uint]
	errno := binary.LittleEndian.Uint16(data[1:3])

	pos := 3

	// SQL State [optional: # + 5bytes string]
	if data[3] == 0x23 {
		//sqlstate := string(data[4 : 4+5])
		pos = 9
	}

	// Error Message [string]
	return &MySQLError{
		Number:  errno,
		Message: string(data[pos:]),
	}
}

func readStatus(b []byte) statusFlag {
	return statusFlag(b[0]) | statusFlag(b[1])<<8
}

// Ok Packet
// http://dev.mysql.com/doc/internals/en/generic-response-packets.html#packet-OK_Packet
func (mc *MysqlConn) handleOkPacket(data []byte) error {
	var n, m int

	// 0x00 [1 byte]

	// Affected rows [Length Coded Binary]
	mc.affectedRows, _, n = readLengthEncodedInteger(data[1:])

	// Insert id [Length Coded Binary]
	mc.insertID, _, m = readLengthEncodedInteger(data[1+n:])

	// server_status [2 bytes]
	mc.status = readStatus(data[1+n+m : 1+n+m+2])
	if err := mc.discardResults(); err != nil {
		return err
	}

	// warning count [2 bytes]
	if !mc.strict {
		return nil
	}

	pos := 1 + n + m + 2
	if binary.LittleEndian.Uint16(data[pos:pos+2]) > 0 {
		return mc.getWarnings()
	}
	return nil
}

// Read Packets as Field Packets until EOF-Packet or an Error appears
// http://dev.mysql.com/doc/internals/en/com-query-response.html#packet-Protocol::ColumnDefinition41
func (mc *MysqlConn) readColumns(count int) ([]mysqlField, error) {
	columns := make([]mysqlField, count)

	for i := 0; ; i++ {
		data, err := mc.readPacket()
		if err != nil {
			return nil, err
		}

		// EOF Packet
		if data[0] == iEOF && (len(data) == 5 || len(data) == 1) {
			if i == count {
				return columns, nil
			}
			return nil, fmt.Errorf("column count mismatch n:%d len:%d", count, len(columns))
		}

		// Catalog
		pos, err := skipLengthEncodedString(data)
		if err != nil {
			return nil, err
		}

		// Database [len coded string]
		n, err := skipLengthEncodedString(data[pos:])
		if err != nil {
			return nil, err
		}
		pos += n

		// Table [len coded string]
		if mc.cfg.ColumnsWithAlias {
			tableName, _, n, err := readLengthEncodedString(data[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
			columns[i].tableName = string(tableName)
		} else {
			n, err = skipLengthEncodedString(data[pos:])
			if err != nil {
				return nil, err
			}
			pos += n
		}

		// Original table [len coded string]
		n, err = skipLengthEncodedString(data[pos:])
		if err != nil {
			return nil, err
		}
		pos += n

		// Name [len coded string]
		name, _, n, err := readLengthEncodedString(data[pos:])
		if err != nil {
			return nil, err
		}
		columns[i].name = string(name)
		pos += n

		// Original name [len coded string]
		n, err = skipLengthEncodedString(data[pos:])
		if err != nil {
			return nil, err
		}

		// Filler [uint8]
		// Charset [charset, collation uint8]
		// Length [uint32]
		pos += n + 1 + 2 + 4

		// Field type [uint8]
		columns[i].fieldType = data[pos]
		pos++

		// Flags [uint16]
		columns[i].flags = fieldFlag(binary.LittleEndian.Uint16(data[pos : pos+2]))
		pos += 2

		// Decimals [uint8]
		columns[i].decimals = data[pos]
		//pos++

		// Default value [len coded binary]
		//if pos < len(data) {
		//	defaultVal, _, err = bytesToLengthCodedBinary(data[pos:])
		//}
	}
}

// Read Packets as Field Packets until EOF-Packet or an Error appears
// http://dev.mysql.com/doc/internals/en/com-query-response.html#packet-ProtocolText::ResultsetRow
func (rows *textRows) readRow(dest []interface{}) error {
	mc := rows.mc

	data, err := mc.readPacket()
	if err != nil {
		return err
	}

	// EOF Packet
	if data[0] == iEOF && len(data) == 5 {
		// server_status [2 bytes]
		rows.mc.status = readStatus(data[3:])
		err = rows.mc.discardResults()
		if err == nil {
			err = io.EOF
		} else {
			// connection unusable
			rows.mc.Close()
		}
		rows.mc = nil
		return err
	}
	if data[0] == iERR {
		rows.mc = nil
		return mc.handleErrorPacket(data)
	}

	// RowSet Packet
	var n int
	var isNull bool
	pos := 0

	for i := range dest {
		// Read bytes and convert to string
		dest[i], isNull, n, err = readLengthEncodedString(data[pos:])
		pos += n
		if err == nil {
			if !isNull {
				if !mc.parseTime {
					continue
				} else {
					switch rows.columns[i].fieldType {
					case fieldTypeTimestamp, fieldTypeDateTime,
						fieldTypeDate, fieldTypeNewDate:
						dest[i], err = parseDateTime(
							string(dest[i].([]byte)),
							mc.cfg.Loc,
						)
						if err == nil {
							continue
						}
					default:
						continue
					}
				}

			} else {
				dest[i] = nil
				continue
			}
		}
		return err // err != nil
	}

	return nil
}

// Reads Packets until EOF-Packet or an Error appears. Returns count of Packets read
func (mc *MysqlConn) readUntilEOF() error {
	for {
		data, err := mc.readPacket()
		if err != nil {
			return err
		}

		switch data[0] {
		case iERR:
			return mc.handleErrorPacket(data)
		case iEOF:
			if len(data) == 5 {
				mc.status = readStatus(data[3:])
			}
			return nil
		}
	}
}

/******************************************************************************
*                           Prepared Statements                               *
******************************************************************************/

func (mc *MysqlConn) discardResults() error {
	for mc.status&statusMoreResultsExists != 0 {
		resLen, err := mc.readResultSetHeaderPacket()
		if err != nil {
			return err
		}
		if resLen > 0 {
			// columns
			if err := mc.readUntilEOF(); err != nil {
				return err
			}
			// rows
			if err := mc.readUntilEOF(); err != nil {
				return err
			}
		} else {
			mc.status &^= statusMoreResultsExists
		}
	}
	return nil
}

// http://dev.mysql.com/doc/internals/en/binary-protocol-resultset-row.html
func (rows *binaryRows) readRow(dest []interface{}) error {
	data, err := rows.mc.readPacket()
	if err != nil {
		return err
	}

	// packet indicator [1 byte]
	if data[0] != iOK {
		// EOF Packet
		if data[0] == iEOF && len(data) == 5 {
			rows.mc.status = readStatus(data[3:])
			err = rows.mc.discardResults()
			if err == nil {
				err = io.EOF
			} else {
				// connection unusable
				rows.mc.Close()
			}
			rows.mc = nil
			return err
		}
		rows.mc = nil

		// Error otherwise
		return rows.mc.handleErrorPacket(data)
	}

	// NULL-bitmap,  [(column-count + 7 + 2) / 8 bytes]
	pos := 1 + (len(dest)+7+2)>>3
	nullMask := data[1:pos]

	for i := range dest {
		// Field is NULL
		// (byte >> bit-pos) % 2 == 1
		if ((nullMask[(i+2)>>3] >> uint((i+2)&7)) & 1) == 1 {
			dest[i] = nil
			continue
		}

		// Convert to byte-coded string
		switch rows.columns[i].fieldType {
		case fieldTypeNULL:
			dest[i] = nil
			continue

		// Numeric Types
		case fieldTypeTiny:
			if rows.columns[i].flags&flagUnsigned != 0 {
				dest[i] = int64(data[pos])
			} else {
				dest[i] = int64(int8(data[pos]))
			}
			pos++
			continue

		case fieldTypeShort, fieldTypeYear:
			if rows.columns[i].flags&flagUnsigned != 0 {
				dest[i] = int64(binary.LittleEndian.Uint16(data[pos : pos+2]))
			} else {
				dest[i] = int64(int16(binary.LittleEndian.Uint16(data[pos : pos+2])))
			}
			pos += 2
			continue

		case fieldTypeInt24, fieldTypeLong:
			if rows.columns[i].flags&flagUnsigned != 0 {
				dest[i] = int64(binary.LittleEndian.Uint32(data[pos : pos+4]))
			} else {
				dest[i] = int64(int32(binary.LittleEndian.Uint32(data[pos : pos+4])))
			}
			pos += 4
			continue

		case fieldTypeLongLong:
			if rows.columns[i].flags&flagUnsigned != 0 {
				val := binary.LittleEndian.Uint64(data[pos : pos+8])
				if val > math.MaxInt64 {
					dest[i] = uint64ToString(val)
				} else {
					dest[i] = int64(val)
				}
			} else {
				dest[i] = int64(binary.LittleEndian.Uint64(data[pos : pos+8]))
			}
			pos += 8
			continue

		case fieldTypeFloat:
			dest[i] = float32(math.Float32frombits(binary.LittleEndian.Uint32(data[pos : pos+4])))
			pos += 4
			continue

		case fieldTypeDouble:
			dest[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[pos : pos+8]))
			pos += 8
			continue

		// Length coded Binary Strings
		case fieldTypeDecimal, fieldTypeNewDecimal, fieldTypeVarChar,
			fieldTypeBit, fieldTypeEnum, fieldTypeSet, fieldTypeTinyBLOB,
			fieldTypeMediumBLOB, fieldTypeLongBLOB, fieldTypeBLOB,
			fieldTypeVarString, fieldTypeString, fieldTypeGeometry, fieldTypeJSON:
			var isNull bool
			var n int
			dest[i], isNull, n, err = readLengthEncodedString(data[pos:])
			pos += n
			if err == nil {
				if !isNull {
					continue
				} else {
					dest[i] = nil
					continue
				}
			}
			return err

		case
			fieldTypeDate, fieldTypeNewDate, // Date YYYY-MM-DD
			fieldTypeTime,                         // Time [-][H]HH:MM:SS[.fractal]
			fieldTypeTimestamp, fieldTypeDateTime: // Timestamp YYYY-MM-DD HH:MM:SS[.fractal]

			num, isNull, n := readLengthEncodedInteger(data[pos:])
			pos += n

			switch {
			case isNull:
				dest[i] = nil
				continue
			case rows.columns[i].fieldType == fieldTypeTime:
				// database/sql does not support an equivalent to TIME, return a string
				var dstlen uint8
				switch decimals := rows.columns[i].decimals; decimals {
				case 0x00, 0x1f:
					dstlen = 8
				case 1, 2, 3, 4, 5, 6:
					dstlen = 8 + 1 + decimals
				default:
					return fmt.Errorf(
						"protocol error, illegal decimals value %d",
						rows.columns[i].decimals,
					)
				}
				dest[i], err = formatBinaryDateTime(data[pos:pos+int(num)], dstlen, true)
			case rows.mc.parseTime:
				dest[i], err = parseBinaryDateTime(num, data[pos:], rows.mc.cfg.Loc)
			default:
				var dstlen uint8
				if rows.columns[i].fieldType == fieldTypeDate {
					dstlen = 10
				} else {
					switch decimals := rows.columns[i].decimals; decimals {
					case 0x00, 0x1f:
						dstlen = 19
					case 1, 2, 3, 4, 5, 6:
						dstlen = 19 + 1 + decimals
					default:
						return fmt.Errorf(
							"protocol error, illegal decimals value %d",
							rows.columns[i].decimals,
						)
					}
				}
				dest[i], err = formatBinaryDateTime(data[pos:pos+int(num)], dstlen, false)
			}

			if err == nil {
				pos += int(num)
				continue
			} else {
				return err
			}

		// Please report if this happens!
		default:
			return fmt.Errorf("unknown field type %d", rows.columns[i].fieldType)
		}
	}

	return nil
}

/******************************************************************************
*                           Dump Statements                                   *
*******************************************************************************/

func (mc *MysqlConn) writeDumpBinlogPosPacket(serverID, offset uint32, filename string, flags uint16) error {
	mc.sequence = 0
	length := 4 + //header
		1 + // ComBinlogDump
		4 + // binlog-pos
		2 + // flags
		4 + // server-id
		len(filename) // binlog-filename
	data := make([]byte, length)
	pos := writeByte(data, 4, comBinlogDump)
	pos = writeUint32(data, pos, uint32(offset))
	pos = writeUint16(data, pos, flags)
	pos = writeUint32(data, pos, serverID)
	pos = writeEOFString(data, pos, filename)

	return mc.writePacket(data)
}

func writeEOFString(data []byte, pos int, value string) int {
	pos += copy(data[pos:], value)
	return pos
}

func writeByte(data []byte, pos int, value byte) int {
	data[pos] = value
	return pos + 1
}

func writeUint16(data []byte, pos int, value uint16) int {
	binary.LittleEndian.PutUint16(data[pos:], value)
	return pos + 2
}

func writeUint32(data []byte, pos int, value uint32) int {
	binary.LittleEndian.PutUint32(data[pos:], value)
	return pos + 4
}
