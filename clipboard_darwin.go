package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

char* getFilePathFromPasteboard() {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    NSArray *classes = @[[NSURL class]];
    NSDictionary *options = @{};
    NSArray *fileURLs = [pasteboard readObjectsForClasses:classes options:options];
    
    if (fileURLs.count > 0) {
        NSURL *url = fileURLs[0];
        if ([url isFileURL]) {
            return strdup([[url path] UTF8String]);
        }
    }
    return NULL;
}
*/
import "C"
import "unsafe"

func getFilePath() string {
	cPath := C.getFilePathFromPasteboard()
	if cPath == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cPath))
	return C.GoString(cPath)
}
