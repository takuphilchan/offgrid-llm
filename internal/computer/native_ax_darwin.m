//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <CommonCrypto/CommonDigest.h>
#import <libproc.h>
#import <sys/proc_info.h>
#import <unistd.h>
#import <string.h>
#import <stdlib.h>

// AX objects remain retained and addressed directly. There is no selector,
// scripting, raw input, shell, or system-wide element-at-coordinate operation.
@interface OGAXReference : NSObject { @public AXUIElementRef element; }
- (id)initWithElement:(AXUIElementRef)value;
@end
@implementation OGAXReference
- (id)initWithElement:(AXUIElementRef)value { if((self=[super init]))element=(AXUIElementRef)CFRetain(value);return self; }
- (void)dealloc { if(element)CFRelease(element);[super dealloc]; }
@end

static NSMutableDictionary *targets,*controls,*prepared;
static NSDictionary *selected,*observation;
static unsigned long long sequence;
static NSString *identifier(void){return [[NSUUID UUID] UUIDString];}
static NSDictionary *failure(NSString *code){return @{ @"error":code };}
static NSDictionary *success(id result){return @{ @"result":result?:[NSNull null] };}
static id attribute(AXUIElementRef element,CFStringRef name){
    CFTypeRef value=NULL;
    if(AXUIElementCopyAttributeValue(element,name,&value)!=kAXErrorSuccess||!value)return nil;
    return [(id)value autorelease];
}
static NSString *stringAttribute(AXUIElementRef element,CFStringRef name){id value=attribute(element,name);return [value isKindOfClass:[NSString class]]?value:@"";}
static NSString *digest(id value){
    NSData *data=[NSJSONSerialization dataWithJSONObject:value options:NSJSONWritingSortedKeys|NSJSONWritingFragmentsAllowed error:nil];
    if(!data)return @"";
    unsigned char bytes[CC_SHA256_DIGEST_LENGTH];CC_SHA256(data.bytes,(CC_LONG)data.length,bytes);
    NSMutableString *result=[NSMutableString string];for(int i=0;i<CC_SHA256_DIGEST_LENGTH;i++)[result appendFormat:@"%02x",bytes[i]];return result;
}
static NSString *quoted(NSString *text){NSData *data=[NSJSONSerialization dataWithJSONObject:text options:NSJSONWritingFragmentsAllowed error:nil];return [[[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding]autorelease]?:@"";}
static NSString *timestamp(void){NSDateFormatter *format=[[[NSDateFormatter alloc]init]autorelease];format.locale=[[[NSLocale alloc]initWithLocaleIdentifier:@"en_US_POSIX"]autorelease];format.timeZone=[NSTimeZone timeZoneForSecondsFromGMT:0];format.dateFormat=@"yyyy-MM-dd'T'HH:mm:ss.SSS'Z'";return [format stringFromDate:[NSDate date]];}
static BOOL consoleAvailable(void){
    NSDictionary *session=(NSDictionary *)CGSessionCopyCurrentDictionary();
    BOOL valid=session&&[session[(NSString *)kCGSessionOnConsoleKey] boolValue]&&[session[(NSString *)kCGSessionLoginDoneKey] boolValue]&&[session[(NSString *)kCGSessionUserIDKey] unsignedIntValue]==getuid();
    // The host consent surface separately revokes on workspace deactivation.
    // A reported lock is a hard denial; this is not a screen-capture permission.
    if([session[@"CGSSessionScreenIsLocked"] boolValue])valid=NO;
    [session release];return valid;
}
static NSString *generation(pid_t pid){
    struct proc_bsdinfo info;
    if(pid==getpid()||proc_pidinfo(pid,PROC_PIDTBSDINFO,0,&info,sizeof(info))!=sizeof(info)||info.pbi_uid!=getuid()||info.pbi_ruid!=getuid()||info.pbi_uid==0)return nil;
    return [NSString stringWithFormat:@"%d:%llu:%llu",pid,(unsigned long long)info.pbi_start_tvsec,(unsigned long long)info.pbi_start_tvusec];
}
static BOOL targetValid(BOOL focus){
    if(!selected||!AXIsProcessTrusted()||!consoleAvailable())return NO;
    pid_t pid=[selected[@"pid"] intValue];NSString *current=generation(pid);
    if(!current||![current isEqual:selected[@"generation"]])return NO;
    OGAXReference *window=selected[@"window"],*app=selected[@"app"];
    id windows=attribute(app->element,kAXWindowsAttribute);BOOL present=NO;
    if([windows isKindOfClass:[NSArray class]])for(id candidate in windows)if(CFEqual((CFTypeRef)candidate,window->element)){present=YES;break;}
    if(!present)return NO;
    if(focus){
        if([NSWorkspace sharedWorkspace].frontmostApplication.processIdentifier!=pid)return NO;
        id focused=attribute(app->element,kAXFocusedWindowAttribute);
        if(!focused||!CFEqual((CFTypeRef)focused,window->element))return NO;
    }
    return YES;
}
static BOOL withinTarget(AXUIElementRef element){
    OGAXReference *window=selected[@"window"];
    AXUIElementRef current=(AXUIElementRef)CFRetain(element);
    for(int depth=0;depth<128;depth++){
        if(CFEqual(current,window->element)){CFRelease(current);return YES;}
        id parent=attribute(current,kAXParentAttribute);CFRelease(current);
        if(!parent||CFGetTypeID((CFTypeRef)parent)!=AXUIElementGetTypeID())return NO;
        current=(AXUIElementRef)CFRetain((CFTypeRef)parent);
    }
    CFRelease(current);return NO;
}
static NSDictionary *inspect(AXUIElementRef element){
    pid_t pid=0;if(AXUIElementGetPid(element,&pid)!=kAXErrorSuccess||pid!=[selected[@"pid"] intValue])return nil;
    NSString *role=stringAttribute(element,kAXRoleAttribute),*subrole=stringAttribute(element,kAXSubroleAttribute);
    if([subrole isEqual:@"AXSecureTextField"]||[role isEqual:@"AXSecureTextField"]||![attribute(element,kAXEnabledAttribute) boolValue])return nil;
    NSString *name=stringAttribute(element,kAXTitleAttribute);if(!name.length)name=stringAttribute(element,kAXDescriptionAttribute);
    NSString *lower=name.lowercaseString;
    for(NSString *word in @[@"password",@"passcode",@"credential",@"credit card",@"verification code",@"security code",@"token"])if([lower containsString:word])return nil;
    if(name.length>1024)name=[name substringToIndex:1024];
    Boolean settable=false;AXUIElementIsAttributeSettable(element,kAXValueAttribute,&settable);
    BOOL writable=settable&&([role isEqual:(NSString *)kAXTextFieldRole]||[role isEqual:(NSString *)kAXTextAreaRole]);
    CFArrayRef names=NULL;BOOL invokable=NO;
    if(AXUIElementCopyActionNames(element,&names)==kAXErrorSuccess&&names){invokable=[(NSArray *)names containsObject:(NSString *)kAXPressAction];CFRelease(names);}
    NSString *value=stringAttribute(element,kAXValueAttribute);
    if(value.length>1048576)return nil;
    return @{ @"name":name,@"role":role,@"writable":@(writable),@"invokable":@(invokable),@"fingerprint":digest(@[name,role,subrole,@(writable),@(invokable),value]) };
}
static NSDictionary *perform(NSDictionary *request){
    NSString *kind=request[@"kind"];id args=request[@"args"];
    if(![kind isKindOfClass:[NSString class]])return failure(@"computer_invalid_action");
    if(!AXIsProcessTrusted())return failure(@"computer_permission_denied");
    if(!consoleAvailable())return failure(@"computer_scope_violation");
    if([kind isEqual:@"permissions"])return success(@YES);
    if(!targets){targets=[NSMutableDictionary new];controls=[NSMutableDictionary new];prepared=[NSMutableDictionary new];}
    if([kind isEqual:@"targets"]){
        if(selected)return failure(@"computer_scope_violation");
        [targets removeAllObjects];NSMutableArray *result=[NSMutableArray array];
        for(NSRunningApplication *running in [NSWorkspace sharedWorkspace].runningApplications){
            if(result.count>=100)break;
            if(running.activationPolicy!=NSApplicationActivationPolicyRegular)continue;
            pid_t pid=running.processIdentifier;NSString *created=generation(pid);if(!created)continue;
            AXUIElementRef app=AXUIElementCreateApplication(pid);AXUIElementSetMessagingTimeout(app,2);
            id windows=attribute(app,kAXWindowsAttribute);
            if([windows isKindOfClass:[NSArray class]])for(id candidate in windows){
                if(result.count>=100)break;
                if(CFGetTypeID((CFTypeRef)candidate)!=AXUIElementGetTypeID())continue;
                AXUIElementRef window=(AXUIElementRef)candidate;
                NSString *title=stringAttribute(window,kAXTitleAttribute);if(!title.length||title.length>1024)continue;
                NSString *key=identifier();
                NSDictionary *identity=@{@"id":key,@"os_session":[NSString stringWithFormat:@"macos:%u:console",getuid()],@"process_generation":created,@"surface":identifier(),@"driver":@"macos-accessibility"};
                NSDictionary *target=@{@"identity":identity,@"title":title};
                targets[key]=@{@"target":target,@"pid":@(pid),@"generation":created,@"window":[[[OGAXReference alloc]initWithElement:window]autorelease],@"app":[[[OGAXReference alloc]initWithElement:app]autorelease]};
                [result addObject:target];
            }
            CFRelease(app);
        }
        return success(result);
    }
    if([kind isEqual:@"select"]){
        NSDictionary *target=targets[args[@"target"]];if(!target||selected)return failure(@"computer_scope_violation");
        selected=[target retain];if(!targetValid(NO)){[selected release];selected=nil;return failure(@"computer_scope_violation");}
        return success(selected[@"target"]);
    }
    if(!targetValid(NO))return failure(@"computer_scope_violation");
    if([kind isEqual:@"focus"]){
        NSRunningApplication *running=[NSRunningApplication runningApplicationWithProcessIdentifier:[selected[@"pid"] intValue]];
        [running activateWithOptions:NSApplicationActivateIgnoringOtherApps];
        OGAXReference *window=selected[@"window"];AXUIElementPerformAction(window->element,kAXRaiseAction);
        return targetValid(YES)?success(@YES):failure(@"computer_scope_violation");
    }
    if([kind isEqual:@"observe"]){
        [controls removeAllObjects];[prepared removeAllObjects];
        NSMutableArray *queue=[NSMutableArray arrayWithObject:selected[@"window"]],*elements=[NSMutableArray array],*fingerprints=[NSMutableArray array];BOOL limited=NO;
        NSUInteger visited=0;
        while(queue.count&&visited<400){
            OGAXReference *reference=[[queue objectAtIndex:0] retain];[queue removeObjectAtIndex:0];visited++;
            NSDictionary *info=inspect(reference->element);
            if(info){NSString *key=identifier();NSMutableDictionary *entry=[info mutableCopy];entry[@"id"]=key;entry[@"control_identity"]=identifier();entry[@"ref"]=reference;controls[key]=entry;
                [elements addObject:@{@"id":key,@"name":info[@"name"],@"role":info[@"role"],@"writable":info[@"writable"],@"invokable":info[@"invokable"]}];[fingerprints addObject:info[@"fingerprint"]];[entry release];}
            CFIndex count=0;if(AXUIElementGetAttributeValueCount(reference->element,kAXChildrenAttribute,&count)==kAXErrorSuccess&&count>0){
                CFIndex allowed=MIN(count,400-(CFIndex)visited-(CFIndex)queue.count);if(allowed<count)limited=YES;
                CFArrayRef children=NULL;
                if(allowed>0&&AXUIElementCopyAttributeValues(reference->element,kAXChildrenAttribute,0,allowed,&children)==kAXErrorSuccess&&children){for(id child in (NSArray *)children)if(CFGetTypeID((CFTypeRef)child)==AXUIElementGetTypeID())[queue addObject:[[[OGAXReference alloc]initWithElement:(AXUIElementRef)child]autorelease]];CFRelease(children);}
            }
            [reference release];
        }
        if(queue.count)limited=YES;
        [observation release];observation=[@{@"id":identifier(),@"target":selected[@"target"][@"identity"],@"sequence":@(++sequence),@"captured_at":timestamp(),@"state_digest":digest(fingerprints)} retain];
        return success(@{@"observation":observation,@"elements":elements,@"limited":@(limited)});
    }
    if([kind isEqual:@"prepare"]){
        NSDictionary *op=args[@"operation"],*observed=args[@"observation"];
        if(!observation||![observed[@"id"] isEqual:observation[@"id"]]||![observed[@"target"] isEqual:observation[@"target"]]||![observed[@"state_digest"] isEqual:observation[@"state_digest"]]||![observed[@"sequence"] isEqual:observation[@"sequence"]])return failure(@"computer_stale_observation");
        NSDictionary *control=controls[op[@"element"]];OGAXReference *reference=control[@"ref"];
        if(!reference||!withinTarget(reference->element))return failure(@"computer_scope_violation");
        NSDictionary *fresh=inspect(reference->element);
        if(!fresh||![fresh[@"fingerprint"] isEqual:control[@"fingerprint"]])return failure(@"computer_stale_observation");
        BOOL replace=[op[@"kind"] isEqual:@"replace_text"],activate=[op[@"kind"] isEqual:@"activate"];
        if((!replace&&!activate)||(replace&&![control[@"writable"] boolValue])||(activate&&![control[@"invokable"] boolValue]))return failure(@"computer_invalid_action");
        NSString *key=identifier();NSDictionary *action=@{@"id":key,@"operation":op,@"control_identity":control[@"control_identity"],@"precondition":control[@"fingerprint"],@"expected_change":digest(op)};prepared[key]=action;return success(action);
    }
    if([kind isEqual:@"approval_summary"]){
        NSMutableString *text=[NSMutableString stringWithFormat:@"Approve these exact changes in %@?\nOffGrid will focus this application.\n\n",quoted(selected[@"target"][@"title"])];
        for(NSDictionary *action in args[@"actions"]){if(![action isEqual:prepared[action[@"id"]]])return failure(@"computer_approval_invalid");NSDictionary *op=action[@"operation"],*control=controls[op[@"element"]];
            if([op[@"kind"] isEqual:@"replace_text"])[text appendFormat:@"Replace all text in %@ with:\n%@\n\n",quoted(control[@"name"]),quoted(op[@"text"])];
            else [text appendFormat:@"Activate %@. This may submit or change information; dispatch alone does not verify the outcome.\n\n",quoted(control[@"name"])];}
        [text appendString:@"Only these changes are allowed. Use the local Stop control to revoke access."];return success(text);
    }
    if([kind isEqual:@"dispatch"]){
        NSDictionary *action=args;if(![action isEqual:prepared[action[@"id"]]])return failure(@"computer_approval_invalid");
        [prepared removeObjectForKey:action[@"id"]];
        if(!targetValid(YES))return failure(@"computer_scope_violation");
        NSDictionary *op=action[@"operation"],*control=controls[op[@"element"]];OGAXReference *reference=control[@"ref"];
        if(!reference||!withinTarget(reference->element))return failure(@"computer_scope_violation");
        NSDictionary *fresh=inspect(reference->element);
        if(!fresh||![fresh[@"fingerprint"] isEqual:action[@"precondition"]]||![control[@"control_identity"] isEqual:action[@"control_identity"]])return failure(@"computer_stale_observation");
        [observation release];observation=nil;
        if([op[@"kind"] isEqual:@"replace_text"]){
            if(AXUIElementSetAttributeValue(reference->element,kAXValueAttribute,(CFTypeRef)op[@"text"])!=kAXErrorSuccess)return failure(@"computer_uncertain_outcome");
            NSString *value=stringAttribute(reference->element,kAXValueAttribute);if(![value isEqual:op[@"text"]])return failure(@"computer_uncertain_outcome");
            return success(@{@"action_id":action[@"id"],@"outcome":@"verified",@"evidence_id":digest(value),@"uncertain":@NO});
        }
        if(AXUIElementPerformAction(reference->element,kAXPressAction)!=kAXErrorSuccess)return failure(@"computer_uncertain_outcome");
        return success(@{@"action_id":action[@"id"],@"outcome":@"dispatched",@"uncertain":@NO});
    }
    return failure(@"computer_invalid_action");
}

char *offgrid_ax_call(const char *request){
    @autoreleasepool {
        @try {
            NSData *data=[[NSString stringWithUTF8String:request] dataUsingEncoding:NSUTF8StringEncoding];
            id value=[NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
            NSDictionary *result=[value isKindOfClass:[NSDictionary class]]?perform(value):failure(@"computer_invalid_action");
            NSData *output=[NSJSONSerialization dataWithJSONObject:result options:NSJSONWritingSortedKeys error:nil];
            NSString *text=[[[NSString alloc]initWithData:output encoding:NSUTF8StringEncoding]autorelease];return strdup((text?:@"{\"error\":\"computer_provider_unavailable\"}").UTF8String);
        } @catch(NSException *exception) {return strdup("{\"error\":\"computer_provider_unavailable\"}");}
    }
}
void offgrid_ax_close(void){[selected release];selected=nil;[observation release];observation=nil;[controls release];controls=nil;[prepared release];prepared=nil;[targets release];targets=nil;}
