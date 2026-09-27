//go:build darwin && cgo

package agent

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

static CFMutableDictionaryRef keyQuery(void) {
  CFMutableDictionaryRef q=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
  CFDictionarySetValue(q,kSecClass,kSecClassGenericPassword);
  CFStringRef service=CFSTR("advent-agent.deepseek");
  CFStringRef account=CFSTR("api");
  CFDictionarySetValue(q,kSecAttrService,service);
  CFDictionarySetValue(q,kSecAttrAccount,account);
  return q;
}
static OSStatus keyGet(char **out, size_t *length) {
  CFMutableDictionaryRef q=keyQuery();
  CFDictionarySetValue(q,kSecReturnData,kCFBooleanTrue);
  CFDictionarySetValue(q,kSecMatchLimit,kSecMatchLimitOne);
  CFTypeRef result=NULL;
  OSStatus status=SecItemCopyMatching(q,&result);
  CFRelease(q);
  if(status!=errSecSuccess) return status;
  CFDataRef data=(CFDataRef)result;
  *length=(size_t)CFDataGetLength(data);
  *out=(char*)malloc(*length+1);
  if(!*out){CFRelease(result);return errSecAllocate;}
  memcpy(*out,CFDataGetBytePtr(data),*length);
  (*out)[*length]=0;
  CFRelease(result);
  return errSecSuccess;
}
static OSStatus keySet(const char *value, size_t length) {
  CFMutableDictionaryRef q=keyQuery();
  CFDataRef data=CFDataCreate(NULL,(const UInt8*)value,(CFIndex)length);
  CFMutableDictionaryRef attrs=CFDictionaryCreateMutable(NULL,0,&kCFTypeDictionaryKeyCallBacks,&kCFTypeDictionaryValueCallBacks);
  CFDictionarySetValue(attrs,kSecValueData,data);
  OSStatus status=SecItemUpdate(q,attrs);
  if(status==errSecItemNotFound){CFDictionarySetValue(q,kSecValueData,data);status=SecItemAdd(q,NULL);}
  CFRelease(attrs);CFRelease(data);CFRelease(q);
  return status;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

func KeychainDeepSeekKey() (string, error) {
	var ptr *C.char
	var length C.size_t
	status := C.keyGet(&ptr, &length)
	if status != 0 {
		return "", fmt.Errorf("Keychain lookup failed (OSStatus %d)", int(status))
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoStringN(ptr, C.int(length)), nil
}
func SetKeychainDeepSeekKey(key string) error {
	if key == "" {
		return errors.New("empty API key")
	}
	p := C.CString(key)
	defer C.free(unsafe.Pointer(p))
	status := C.keySet(p, C.size_t(len(key)))
	if status != 0 {
		return fmt.Errorf("Keychain write failed (OSStatus %d)", int(status))
	}
	return nil
}
