package main

/*
#cgo LDFLAGS: -lole32 -luuid
#include <windows.h>
#include <shlobj.h>

char* getFilePathFromClipboard() {
    if (!OpenClipboard(NULL)) {
        return NULL;
    }
    
    HDROP hDrop = (HDROP)GetClipboardData(CF_HDROP);
    if (hDrop == NULL) {
        CloseClipboard();
        return NULL;
    }
    
    UINT fileCount = DragQueryFileW(hDrop, 0xFFFFFFFF, NULL, 0);
    if (fileCount > 0) {
        WCHAR filePath[MAX_PATH];
        if (DragQueryFileW(hDrop, 0, filePath, MAX_PATH) > 0) {
            CloseClipboard();
            
            // 转换 WCHAR 到 UTF-8
            int size = WideCharToMultiByte(CP_UTF8, 0, filePath, -1, NULL, 0, NULL, NULL);
            char* utf8Path = (char*)malloc(size);
            WideCharToMultiByte(CP_UTF8, 0, filePath, -1, utf8Path, size, NULL, NULL);
            return utf8Path;
        }
    }
    
    CloseClipboard();
    return NULL;
}
*/
import "C"
import "unsafe"

func getFilePath() string {
	cPath := C.getFilePathFromClipboard()
	if cPath == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cPath))
	return C.GoString(cPath)
}
