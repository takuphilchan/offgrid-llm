#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>
#import <unistd.h>
#import <arpa/inet.h>

// GUI-only helper. No AX/input API, shell, interpreter, service credentials or
// network listener exists here. stdin/stdout are private inherited host pipes.
static NSLock *outputLock;
static BOOL stopped=NO;
static EventHotKeyRef emergencyKey;
static void sendMessage(NSDictionary *message){
    NSData *body=[NSJSONSerialization dataWithJSONObject:message options:0 error:nil];
    if(!body||body.length>1048576)return;
    uint32_t length=htonl((uint32_t)body.length);
    [outputLock lock];
    @try{[[NSFileHandle fileHandleWithStandardOutput]writeData:[NSData dataWithBytes:&length length:4]];[[NSFileHandle fileHandleWithStandardOutput]writeData:body];}@catch(NSException *exception){}
    [outputLock unlock];
}
static void stopControl(void){if(stopped)return;stopped=YES;sendMessage(@{@"kind":@"stop"});[NSApp terminate:nil];}
static OSStatus emergencyHandler(EventHandlerCallRef next,EventRef event,void *data){stopControl();return noErr;}
static NSData *readExact(NSFileHandle *input,NSUInteger count){NSMutableData *result=[NSMutableData data];while(result.length<count){NSData *part=[input readDataOfLength:count-result.length];if(!part.length)return nil;[result appendData:part];}return result;}

@interface OffGridConsentDelegate : NSObject <NSApplicationDelegate,NSWindowDelegate>
@property(strong) NSWindow *window;
@end
@implementation OffGridConsentDelegate
- (void)stop:(id)sender{stopControl();}
- (BOOL)windowShouldClose:(NSWindow *)window{stopControl();return NO;}
- (void)applicationDidFinishLaunching:(NSNotification *)notice{
    self.window=[[NSWindow alloc]initWithContentRect:NSMakeRect(80,80,580,140) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];
    self.window.title=@"OffGrid computer control";self.window.level=NSFloatingWindowLevel;self.window.delegate=self;
    NSTextField *label=[NSTextField labelWithString:@"Control is supervised. Control+Option+Command+F12 stops."];
    label.frame=NSMakeRect(20,92,540,24);[self.window.contentView addSubview:label];
    NSButton *button=[NSButton buttonWithTitle:@"Stop computer control" target:self action:@selector(stop:)];button.frame=NSMakeRect(20,26,540,48);button.keyEquivalent=@"\033";[self.window.contentView addSubview:button];
    EventTypeSpec type={kEventClassKeyboard,kEventHotKeyPressed};
    if(InstallApplicationEventHandler(emergencyHandler,1,&type,NULL,NULL)!=noErr){stopControl();return;}
    EventHotKeyID identity={'OGST',1};
    if(RegisterEventHotKey(kVK_F12,controlKey|optionKey|cmdKey,identity,GetApplicationEventTarget(),0,&emergencyKey)!=noErr){stopControl();return;}
    [[[NSWorkspace sharedWorkspace]notificationCenter]addObserverForName:NSWorkspaceSessionDidResignActiveNotification object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *note){stopControl();}];
    [self.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];sendMessage(@{@"kind":@"ready"});
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{
        @autoreleasepool {
            NSFileHandle *input=[NSFileHandle fileHandleWithStandardInput];
            @try {
                while(YES){
                    NSData *header=readExact(input,4);if(!header)break;uint32_t size;[header getBytes:&size length:4];size=ntohl(size);if(!size||size>1048576)break;
                    NSData *body=readExact(input,size);if(!body)break;
                    id request=[NSJSONSerialization JSONObjectWithData:body options:0 error:nil];
                    if(![request isKindOfClass:[NSDictionary class]]||[request count]!=3||![request[@"kind"] isEqual:@"confirm"]||![request[@"id"] isKindOfClass:[NSString class]]||![request[@"text"] isKindOfClass:[NSString class]])break;
                    if([request[@"id"] length]>128||[request[@"text"] length]>200000)break;
                    dispatch_sync(dispatch_get_main_queue(),^{
                        if(stopped)return;
                        NSAlert *alert=[NSAlert new];alert.messageText=@"OffGrid computer access";alert.informativeText=@"Approve only the exact scope or changes shown below.";
                        [alert addButtonWithTitle:@"Deny"];[alert addButtonWithTitle:@"Approve"];
                        NSScrollView *scroll=[[NSScrollView alloc]initWithFrame:NSMakeRect(0,0,540,280)];scroll.hasVerticalScroller=YES;
                        NSTextView *text=[[NSTextView alloc]initWithFrame:NSMakeRect(0,0,520,280)];text.string=request[@"text"];text.editable=NO;text.selectable=YES;text.font=[NSFont systemFontOfSize:14];text.verticallyResizable=YES;text.autoresizingMask=NSViewWidthSizable;scroll.documentView=text;alert.accessoryView=scroll;
                        [NSApp activateIgnoringOtherApps:YES];BOOL allowed=[alert runModal]==NSAlertSecondButtonReturn;
                        if(!stopped)sendMessage(@{@"kind":@"consent",@"id":request[@"id"],@"allowed":@(allowed)});
                    });
                }
            } @catch(NSException *exception){}
            dispatch_async(dispatch_get_main_queue(),^{stopControl();});
        }
    });
}
- (void)applicationWillTerminate:(NSNotification *)notice{if(emergencyKey)UnregisterEventHotKey(emergencyKey);}
@end

int main(void){@autoreleasepool{outputLock=[NSLock new];NSApplication *app=[NSApplication sharedApplication];[app setActivationPolicy:NSApplicationActivationPolicyAccessory];OffGridConsentDelegate *delegate=[OffGridConsentDelegate new];app.delegate=delegate;[app run];}return 0;}
