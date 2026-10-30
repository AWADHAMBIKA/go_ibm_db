//go:build linux && (amd64 || arm64) && !db2cgo

package api

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/ibmdb/go_ibm_db/internal/clidriver"
	trc "github.com/ibmdb/go_ibm_db/log2"
)

// Bound Db2 CLI entry points. Pointers are passed as unsafe.Pointer and handles
// as uintptr; small integers are widened and the result is truncated to SQLRETURN.
var (
	fnSQLAllocHandle     func(handleType int32, input uintptr, output unsafe.Pointer) int
	fnSQLBindCol         func(stmt uintptr, col uint32, ctype int32, target unsafe.Pointer, buflen int32, ind unsafe.Pointer) int
	fnSQLBindParameter   func(stmt uintptr, num uint32, ioType int32, valueType int32, paramType int32, colSize uint32, digits int32, value unsafe.Pointer, buflen int32, ind unsafe.Pointer) int
	fnSQLCloseCursor     func(stmt uintptr) int
	fnSQLDescribeColW    func(stmt uintptr, col uint32, name unsafe.Pointer, buflen int32, nameLen, dataType unsafe.Pointer, colSize, digits, nullable unsafe.Pointer) int
	fnSQLDescribeParam   func(stmt uintptr, num uint32, dataType, paramSize, digits, nullable unsafe.Pointer) int
	fnSQLDisconnect      func(dbc uintptr) int
	fnSQLDriverConnectW  func(dbc uintptr, wnd uintptr, in unsafe.Pointer, inLen int32, out unsafe.Pointer, outLen int32, outLenPtr unsafe.Pointer, completion uint32) int
	fnSQLEndTran         func(handleType int32, handle uintptr, completion int32) int
	fnSQLExecute         func(stmt uintptr) int
	fnSQLFetch           func(stmt uintptr) int
	fnSQLFreeHandle      func(handleType int32, handle uintptr) int
	fnSQLGetData         func(stmt uintptr, col uint32, ctype int32, target unsafe.Pointer, buflen int32, ind unsafe.Pointer) int
	fnSQLGetDiagRecW     func(handleType int32, handle uintptr, rec int32, state, native, msg unsafe.Pointer, buflen int32, textLen unsafe.Pointer) int
	fnSQLGetInfo         func(dbc uintptr, infoType uint32, value unsafe.Pointer, buflen int32, lenPtr unsafe.Pointer) int
	fnSQLNumParams       func(stmt uintptr, count unsafe.Pointer) int
	fnSQLNumResultCols   func(stmt uintptr, count unsafe.Pointer) int
	fnSQLPrepareW        func(stmt uintptr, text unsafe.Pointer, textLen int32) int
	fnSQLRowCount        func(stmt uintptr, count unsafe.Pointer) int
	fnSQLSetEnvAttr      func(env uintptr, attr int32, value uintptr, strLen int32) int
	fnSQLSetConnectAttrW func(dbc uintptr, attr int32, value uintptr, strLen int32) int
	fnSQLSetEnvAttrPtr   func(env uintptr, attr int32, value unsafe.Pointer, strLen int32) int
	fnSQLSetConnAttrWPtr func(dbc uintptr, attr int32, value unsafe.Pointer, strLen int32) int
	fnSQLColAttribute    func(stmt uintptr, col uint32, field uint32, charAttr unsafe.Pointer, buflen int32, lenPtr unsafe.Pointer, numAttr unsafe.Pointer) int
	fnSQLMoreResults     func(stmt uintptr) int
	fnSQLSetStmtAttrW    func(stmt uintptr, attr int32, value unsafe.Pointer, strLen int32) int
	fnSQLCreateDbW       func(dbc uintptr, db unsafe.Pointer, dbLen int32, cs unsafe.Pointer, csLen int32, mode unsafe.Pointer, modeLen int32) int
	fnSQLDropDbW         func(dbc uintptr, db unsafe.Pointer, dbLen int32) int
	fnSQLExecDirectW     func(stmt uintptr, text unsafe.Pointer, textLen int32) int
)

var (
	loadMu  sync.Mutex
	loaded  bool
	loadErr error
)

func init() {
	LoadError = func() error {
		loadMu.Lock()
		defer loadMu.Unlock()
		return loadErr
	}
}

// ensureLoaded locates and loads the CLI library and binds all entry points.
// Success is final; a failure is retried on the next call.
func ensureLoaded() bool {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded {
		return true
	}
	if loadErr = loadLibrary(); loadErr != nil {
		trc.Trace1(fmt.Sprintf("api: loading Db2 CLI failed: %v", loadErr))
		return false
	}
	loaded = true
	return true
}

func loadLibrary() error {
	r := clidriver.Resolve()
	if r.Lib == "" {
		// Fall back to the system library search path before downloading.
		if lib, err := purego.Dlopen(clidriver.LibName(), purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
			return bind(lib)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var err error
		if r, err = clidriver.Ensure(ctx); err != nil {
			return err
		}
	}
	lib, err := purego.Dlopen(r.Lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("loading %s failed: %w\nSet %s to a valid Db2 CLI driver directory or remove %s", r.Lib, err, clidriver.EnvHome, r.Home)
	}
	return bind(lib)
}

func bind(lib uintptr) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("binding Db2 CLI functions: %v", p)
		}
	}()
	reg := func(fn any, name string) { purego.RegisterLibFunc(fn, lib, name) }
	reg(&fnSQLAllocHandle, "SQLAllocHandle")
	reg(&fnSQLBindCol, "SQLBindCol")
	reg(&fnSQLBindParameter, "SQLBindParameter")
	reg(&fnSQLCloseCursor, "SQLCloseCursor")
	reg(&fnSQLDescribeColW, "SQLDescribeColW")
	reg(&fnSQLDescribeParam, "SQLDescribeParam")
	reg(&fnSQLDisconnect, "SQLDisconnect")
	reg(&fnSQLDriverConnectW, "SQLDriverConnectW")
	reg(&fnSQLEndTran, "SQLEndTran")
	reg(&fnSQLExecute, "SQLExecute")
	reg(&fnSQLFetch, "SQLFetch")
	reg(&fnSQLFreeHandle, "SQLFreeHandle")
	reg(&fnSQLGetData, "SQLGetData")
	reg(&fnSQLGetDiagRecW, "SQLGetDiagRecW")
	reg(&fnSQLGetInfo, "SQLGetInfo")
	reg(&fnSQLNumParams, "SQLNumParams")
	reg(&fnSQLNumResultCols, "SQLNumResultCols")
	reg(&fnSQLPrepareW, "SQLPrepareW")
	reg(&fnSQLRowCount, "SQLRowCount")
	reg(&fnSQLSetEnvAttr, "SQLSetEnvAttr")
	reg(&fnSQLSetConnectAttrW, "SQLSetConnectAttrW")
	// The *Ptr variables bind the same C symbols with a pointer-typed value
	// argument. They are independent function values, so there is no "last one
	// wins"; they exist because the value is either an integer or a pointer.
	reg(&fnSQLSetEnvAttrPtr, "SQLSetEnvAttr")
	reg(&fnSQLSetConnAttrWPtr, "SQLSetConnectAttrW")
	reg(&fnSQLColAttribute, "SQLColAttribute")
	reg(&fnSQLMoreResults, "SQLMoreResults")
	reg(&fnSQLSetStmtAttrW, "SQLSetStmtAttrW")
	reg(&fnSQLCreateDbW, "SQLCreateDbW")
	reg(&fnSQLDropDbW, "SQLDropDbW")
	reg(&fnSQLExecDirectW, "SQLExecDirectW")
	return nil
}

func ret16(r int) SQLRETURN { return SQLRETURN(int16(r)) }

func SQLAllocHandle(handleType SQLSMALLINT, inputHandle SQLHANDLE, outputHandle *SQLHANDLE) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLAllocHandle() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	r := ret16(fnSQLAllocHandle(int32(handleType), uintptr(inputHandle), unsafe.Pointer(outputHandle)))
	trc.Trace1(fmt.Sprintf("handleType=%d r = %d", handleType, r))
	return r
}

func SQLBindCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr []byte, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLBindCol() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	r := ret16(fnSQLBindCol(uintptr(statementHandle), uint32(columnNumber), int32(targetType), unsafe.Pointer(&targetValuePtr[0]), int32(bufferLength), unsafe.Pointer(vallen)))
	trc.Trace1(fmt.Sprintf("columnNumber=%d, targetType=%d, bufferLength=%d r = %d", columnNumber, targetType, bufferLength, r))
	return r
}

func SQLBindParameter(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, inputOutputType SQLSMALLINT, valueType SQLSMALLINT, parameterType SQLSMALLINT, columnSize SQLULEN, decimalDigits SQLSMALLINT, parameterValue SQLPOINTER, bufferLength SQLLEN, ind *SQLLEN) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLBindParameter() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	r := ret16(fnSQLBindParameter(uintptr(statementHandle), uint32(parameterNumber), int32(inputOutputType), int32(valueType), int32(parameterType), uint32(columnSize), int32(decimalDigits), unsafe.Pointer(parameterValue), int32(bufferLength), unsafe.Pointer(ind)))
	trc.Trace1(fmt.Sprintf("parameterNumber=%d, inputOutputType=%d, valueType=%d, parameterType=%d, columnSize=%d, decimalDigits=%d, bufferLength=%d r = %d", parameterNumber, inputOutputType, valueType, parameterType, columnSize, decimalDigits, bufferLength, r))
	return r
}

func SQLCloseCursor(statementHandle SQLHSTMT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLCloseCursor() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLCloseCursor(uintptr(statementHandle)))
}

func SQLDescribeCol(statementHandle SQLHSTMT, columnNumber SQLUSMALLINT, columnName *SQLWCHAR, bufferLength SQLSMALLINT, nameLengthPtr *SQLSMALLINT, dataTypePtr *SQLSMALLINT, columnSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLDescribeCol() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLDescribeColW(uintptr(statementHandle), uint32(columnNumber), unsafe.Pointer(columnName), int32(bufferLength), unsafe.Pointer(nameLengthPtr), unsafe.Pointer(dataTypePtr), unsafe.Pointer(columnSizePtr), unsafe.Pointer(decimalDigitsPtr), unsafe.Pointer(nullablePtr)))
}

func SQLDescribeParam(statementHandle SQLHSTMT, parameterNumber SQLUSMALLINT, dataTypePtr *SQLSMALLINT, parameterSizePtr *SQLULEN, decimalDigitsPtr *SQLSMALLINT, nullablePtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLDescribeParam() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLDescribeParam(uintptr(statementHandle), uint32(parameterNumber), unsafe.Pointer(dataTypePtr), unsafe.Pointer(parameterSizePtr), unsafe.Pointer(decimalDigitsPtr), unsafe.Pointer(nullablePtr)))
}

func SQLDisconnect(connectionHandle SQLHDBC) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLDisconnect() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLDisconnect(uintptr(connectionHandle)))
}

func SQLDriverConnect(connectionHandle SQLHDBC, windowHandle SQLHWND, inConnectionString *SQLWCHAR, stringLength1 SQLSMALLINT, outConnectionString *SQLWCHAR, bufferLength SQLSMALLINT, stringLength2Ptr *SQLSMALLINT, driverCompletion SQLUSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLDriverConnect() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLDriverConnectW(uintptr(connectionHandle), uintptr(windowHandle), unsafe.Pointer(inConnectionString), int32(stringLength1), unsafe.Pointer(outConnectionString), int32(bufferLength), unsafe.Pointer(stringLength2Ptr), uint32(driverCompletion)))
}

func SQLEndTran(handleType SQLSMALLINT, handle SQLHANDLE, completionType SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLEndTran() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLEndTran(int32(handleType), uintptr(handle), int32(completionType)))
}

func SQLExecute(statementHandle SQLHSTMT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLExecute() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLExecute(uintptr(statementHandle)))
}

func SQLFetch(statementHandle SQLHSTMT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLFetch() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLFetch(uintptr(statementHandle)))
}

func SQLFreeHandle(handleType SQLSMALLINT, handle SQLHANDLE) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLFreeHandle() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLFreeHandle(int32(handleType), uintptr(handle)))
}

func SQLGetData(statementHandle SQLHSTMT, colOrParamNum SQLUSMALLINT, targetType SQLSMALLINT, targetValuePtr SQLPOINTER, bufferLength SQLLEN, vallen *SQLLEN) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLGetData() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLGetData(uintptr(statementHandle), uint32(colOrParamNum), int32(targetType), unsafe.Pointer(targetValuePtr), int32(bufferLength), unsafe.Pointer(vallen)))
}

func SQLGetDiagRec(handleType SQLSMALLINT, handle SQLHANDLE, recNumber SQLSMALLINT, sqlState *SQLWCHAR, nativeErrorPtr *SQLINTEGER, messageText *SQLWCHAR, bufferLength SQLSMALLINT, textLengthPtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLGetDiagRec() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLGetDiagRecW(int32(handleType), uintptr(handle), int32(recNumber), unsafe.Pointer(sqlState), unsafe.Pointer(nativeErrorPtr), unsafe.Pointer(messageText), int32(bufferLength), unsafe.Pointer(textLengthPtr)))
}

func SQLGetInfo(connectionHandle SQLHDBC, infoType SQLUSMALLINT, infoValuePtr SQLPOINTER, bufferLength SQLSMALLINT, stringLengthPtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLGetInfo() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLGetInfo(uintptr(connectionHandle), uint32(infoType), unsafe.Pointer(infoValuePtr), int32(bufferLength), unsafe.Pointer(stringLengthPtr)))
}

func SQLNumParams(statementHandle SQLHSTMT, parameterCountPtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLNumParams() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLNumParams(uintptr(statementHandle), unsafe.Pointer(parameterCountPtr)))
}

func SQLNumResultCols(statementHandle SQLHSTMT, columnCountPtr *SQLSMALLINT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLNumResultCols() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLNumResultCols(uintptr(statementHandle), unsafe.Pointer(columnCountPtr)))
}

func SQLPrepare(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLPrepare() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLPrepareW(uintptr(statementHandle), unsafe.Pointer(statementText), int32(textLength)))
}

func SQLRowCount(statementHandle SQLHSTMT, rowCountPtr *SQLLEN) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLRowCount() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLRowCount(uintptr(statementHandle), unsafe.Pointer(rowCountPtr)))
}

func SQLSetEnvAttr(environmentHandle SQLHENV, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLSetEnvAttr() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLSetEnvAttrPtr(uintptr(environmentHandle), int32(attribute), unsafe.Pointer(valuePtr), int32(stringLength)))
}

func SQLSetConnectAttr(connectionHandle SQLHDBC, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLSetConnectAttr() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLSetConnAttrWPtr(uintptr(connectionHandle), int32(attribute), unsafe.Pointer(valuePtr), int32(stringLength)))
}

func SQLColAttribute(statementHandle SQLHSTMT, ColumnNumber SQLUSMALLINT, FieldIdentifier SQLUSMALLINT, CharacterAttributePtr SQLPOINTER, BufferLength SQLSMALLINT, StringLengthPtr *SQLSMALLINT, NumericAttributePtr SQLPOINTER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLColAttribute() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLColAttribute(uintptr(statementHandle), uint32(ColumnNumber), uint32(FieldIdentifier), unsafe.Pointer(CharacterAttributePtr), int32(BufferLength), unsafe.Pointer(StringLengthPtr), unsafe.Pointer(NumericAttributePtr)))
}

func SQLMoreResults(statementHandle SQLHSTMT) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLMoreResults() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLMoreResults(uintptr(statementHandle)))
}

func SQLSetStmtAttr(statementHandle SQLHSTMT, attribute SQLINTEGER, valuePtr SQLPOINTER, stringLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLSetStmtAttr() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLSetStmtAttrW(uintptr(statementHandle), int32(attribute), unsafe.Pointer(valuePtr), int32(stringLength)))
}

func SQLCreateDb(connectionHandle SQLHDBC, dbnamePtr *SQLWCHAR, dbnameLen SQLINTEGER, codeSetPtr *SQLWCHAR, codeSetLen SQLINTEGER, modePtr *SQLWCHAR, modeLen SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLCreateDb() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLCreateDbW(uintptr(connectionHandle), unsafe.Pointer(dbnamePtr), int32(dbnameLen), unsafe.Pointer(codeSetPtr), int32(codeSetLen), unsafe.Pointer(modePtr), int32(modeLen)))
}

func SQLDropDb(connectionHandle SQLHDBC, dbnamePtr *SQLWCHAR, dbnameLen SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLDropDb() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLDropDbW(uintptr(connectionHandle), unsafe.Pointer(dbnamePtr), int32(dbnameLen)))
}

func SQLExecDirect(statementHandle SQLHSTMT, statementText *SQLWCHAR, textLength SQLINTEGER) (ret SQLRETURN) {
	trc.Trace1("api/zapi_linux_nocgo.go SQLExecDirect() - ENTRY")
	if !ensureLoaded() {
		return SQL_INVALID_HANDLE
	}
	return ret16(fnSQLExecDirectW(uintptr(statementHandle), unsafe.Pointer(statementText), int32(textLength)))
}
