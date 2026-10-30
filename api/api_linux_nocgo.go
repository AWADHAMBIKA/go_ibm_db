// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux && (amd64 || arm64) && !db2cgo

package api

import (
	"fmt"
	"unsafe"

	trc "github.com/ibmdb/go_ibm_db/log2"
)

// Constants are declared directly (no sqlcli.h) and are verified against the
// shipped headers by consts_cgo_test.go.
const (
	SQL_OV_ODBC3 = uintptr(3)

	SQL_ATTR_ODBC_VERSION = 200

	SQL_DRIVER_NOPROMPT = 0

	SQL_HANDLE_ENV  = 1
	SQL_HANDLE_DBC  = 2
	SQL_HANDLE_STMT = 3

	SQL_SUCCESS            = 0
	SQL_SUCCESS_WITH_INFO  = 1
	SQL_INVALID_HANDLE     = -2
	SQL_NO_DATA            = 100
	SQL_NO_TOTAL           = -4
	SQL_NTS                = -3
	SQL_MAX_MESSAGE_LENGTH = 1024
	SQL_NULL_HANDLE        = uintptr(0)
	SQL_NULL_HENV          = uintptr(0)
	SQL_NULL_HDBC          = uintptr(0)
	SQL_NULL_HSTMT         = uintptr(0)

	SQL_PARAM_INPUT        = 1
	SQL_PARAM_OUTPUT       = 4
	SQL_PARAM_INPUT_OUTPUT = 2

	SQL_NULL_DATA    = -1
	SQL_DATA_AT_EXEC = -2

	SQL_UNKNOWN_TYPE                 = 0
	SQL_CHAR                         = 1
	SQL_NUMERIC                      = 2
	SQL_DECIMAL                      = 3
	SQL_INTEGER                      = 4
	SQL_SMALLINT                     = 5
	SQL_FLOAT                        = 6
	SQL_REAL                         = 7
	SQL_DOUBLE                       = 8
	SQL_DATETIME                     = 9
	SQL_DATE                         = 9
	SQL_TIME                         = 10
	SQL_VARCHAR                      = 12
	SQL_TYPE_DATE                    = 91
	SQL_TYPE_TIME                    = 92
	SQL_TYPE_TIMESTAMP               = 93
	SQL_TIMESTAMP                    = 11
	SQL_TYPE_TIMESTAMP_WITH_TIMEZONE = 95
	SQL_LONGVARCHAR                  = -1
	SQL_BINARY                       = -2
	SQL_VARBINARY                    = -3
	SQL_LONGVARBINARY                = -4
	SQL_BIGINT                       = -5
	SQL_TINYINT                      = -6
	SQL_BIT                          = -7
	SQL_WCHAR                        = -8
	SQL_WVARCHAR                     = -9
	SQL_WLONGVARCHAR                 = -10
	SQL_GUID                         = -11
	SQL_BLOB                         = -98
	SQL_CLOB                         = -99
	SQL_SIGNED_OFFSET                = -20
	SQL_UNSIGNED_OFFSET              = -22
	SQL_DBCLOB                       = -350
	SQL_BOOLEAN                      = 16
	SQL_XML                          = -370

	SQL_SS_XML = -152

	SQL_C_CHAR                  = 1
	SQL_C_LONG                  = 4
	SQL_C_SHORT                 = 5
	SQL_C_FLOAT                 = 7
	SQL_C_DOUBLE                = 8
	SQL_C_NUMERIC               = 2
	SQL_C_DATE                  = 9
	SQL_C_TIME                  = 10
	SQL_C_TYPE_TIMESTAMP        = 93
	SQL_C_TIMESTAMP             = 11
	SQL_C_TYPE_TIMESTAMP_EXT_TZ = -364
	SQL_C_BINARY                = -2
	SQL_C_BIT                   = -7
	SQL_C_WCHAR                 = -8
	SQL_C_DEFAULT               = 99
	SQL_C_SBIGINT               = -25
	SQL_C_UBIGINT               = -27
	SQL_C_GUID                  = -11
	SQL_C_DBCHAR                = -350
	SQL_C_TYPE_DATE             = 91
	SQL_C_TYPE_TIME             = 92
	SQL_C_XML                   = -370

	SQL_COMMIT   = 0
	SQL_ROLLBACK = 1

	SQL_AUTOCOMMIT         = 102
	SQL_ATTR_AUTOCOMMIT    = 102
	SQL_AUTOCOMMIT_OFF     = 0
	SQL_AUTOCOMMIT_ON      = 1
	SQL_AUTOCOMMIT_DEFAULT = 1
	SQL_DESC_PRECISION     = 1005
	SQL_DESC_SCALE         = 1006
	SQL_DESC_LENGTH        = 1003
	SQL_DESC_CONCISE_TYPE  = 2
	SQL_DESC_TYPE_NAME     = 14
	SQL_COLUMN_TYPE        = 2
	SQL_COLUMN_TYPE_NAME   = 14
	MAX_FIELD_SIZE         = 1024
	SQL_DESC_NULLABLE      = 1008
	SQL_NULLABLE           = 1
	SQL_NO_NULLS           = 0
	SQL_DECFLOAT           = -360
	SQL_ATTR_PARAMSET_SIZE = 22

	SQL_DRIVER_NAME   = 6
	SQL_DRIVER_VER    = 7
	SQL_ODBC_VER      = 10
	SQL_SERVER_NAME   = 13
	SQL_DATABASE_NAME = 16
	SQL_DBMS_NAME     = 17
	SQL_DBMS_VER      = 18
	SQL_USER_NAME     = 47

	SQL_ATTR_ROW_ARRAY_SIZE   = 27
	SQL_ATTR_ROW_STATUS_PTR   = 25
	SQL_ATTR_ROWS_FETCHED_PTR = 26

	SQL_ROW_SUCCESS           = 0
	SQL_ROW_SUCCESS_WITH_INFO = 6
	SQL_ROW_NOROW             = 3
	SQL_ROW_ERROR             = 5

	SQL_IS_UINTEGER = -5
	SQL_IS_INTEGER  = -6

	//Connection pooling
	SQL_ATTR_CONNECTION_POOLING = 201
	SQL_ATTR_CP_MATCH           = 202
	SQL_CP_OFF                  = uintptr(0)
	SQL_CP_ONE_PER_DRIVER       = uintptr(1)
	SQL_CP_ONE_PER_HENV         = uintptr(2)
	SQL_CP_DEFAULT              = SQL_CP_OFF
	SQL_CP_STRICT_MATCH         = uintptr(0)
	SQL_CP_RELAXED_MATCH        = uintptr(1)
)

// Sizes follow sqlcli.h as compiled without ODBC64 (which the cgo build uses):
// SQLLEN/SQLULEN are 32-bit, like SQLINTEGER/SQLUINTEGER, and SQLWCHAR is a
// 16-bit unit. Using 64-bit here makes Db2 write 4-byte indicators that Go
// would read as 8 bytes.
type (
	SQLHANDLE uintptr
	SQLHENV   uintptr
	SQLHDBC   uintptr
	SQLHSTMT  uintptr
	SQLHWND   uintptr

	SQLWCHAR     uint16
	SQLSCHAR     int8
	SQLSMALLINT  int16
	SQLUSMALLINT uint16
	SQLINTEGER   int32
	SQLUINTEGER  uint32
	SQLPOINTER   unsafe.Pointer
	SQLRETURN    int16

	SQLLEN  int32
	SQLULEN uint32
)

func SQLSetEnvUIntPtrAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr uintptr, stringLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/api_linux_nocgo.go SQLSetEnvUIntPtrAttr() - ENTRY")
	trc.Trace1(fmt.Sprintf("attribute=%d, valuePtr=0x%x, stringLength=%d", attribute, valuePtr, stringLength))

	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	r := fnSQLSetEnvAttr(uintptr(environmentHandle), int32(attribute), valuePtr, int32(stringLength))

	trc.Trace1(fmt.Sprintf("r = %d", int16(r)))
	trc.Trace1("api/api_linux_nocgo.go SQLSetEnvUIntPtrAttr() - EXIT")
	return SQLRETURN(int16(r))
}

func SQLSetConnectUIntPtrAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr uintptr, stringLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/api_linux_nocgo.go SQLSetConnectUIntPtrAttr() - ENTRY")
	trc.Trace1(fmt.Sprintf("attribute=%d, valuePtr=%x, stringLength=%d", attribute, valuePtr, stringLength))

	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	r := fnSQLSetConnectAttrW(uintptr(connectionHandle), int32(attribute), valuePtr, int32(stringLength))

	trc.Trace1(fmt.Sprintf("r = %d", int16(r)))
	trc.Trace1("api/api_linux_nocgo.go SQLSetConnectUIntPtrAttr() - EXIT")
	return SQLRETURN(int16(r))
}
