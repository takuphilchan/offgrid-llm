// Owned AppKit fixture. No files, personal profiles, AppleScript or native input
// directed at other processes. Build/run only through the isolated test runner.
#import <AppKit/AppKit.h>
#import <unistd.h>
#include <stdio.h>

@interface OGDocumentFixture : NSObject <NSApplicationDelegate,NSTabViewDelegate,NSTextViewDelegate>
@property(nonatomic,strong) NSWindow *window;
@property(nonatomic,strong) NSTabView *tabs;
@property(nonatomic,strong) NSTextView *editor;
@property(nonatomic,strong) NSTextField *focusSink;
@property(nonatomic,strong) NSMutableArray<NSMutableDictionary *> *documents;
@property(nonatomic,strong) NSMutableData *pending;
@property NSInteger current;
@property NSInteger nextID;
@property NSInteger delayMS;
@property NSInteger partialBudget;
@property BOOL loading;
@property BOOL creating;
@end

@implementation OGDocumentFixture
- (void)storeDocument {
    if (!self.editor || self.current<0 || self.current>=self.documents.count) return;
    NSMutableDictionary *document=self.documents[self.current];
    document[@"text"]=[self.editor.string copy];
    NSRange range=self.editor.selectedRange;
    document[@"start"]=@(range.location);document[@"end"]=@(NSMaxRange(range));
}
- (void)loadDocument {
    self.loading=YES;
    NSDictionary *document=self.documents[self.current];
    self.editor.string=document[@"text"];
    NSUInteger start=[document[@"start"] unsignedIntegerValue],end=[document[@"end"] unsignedIntegerValue];
    self.editor.selectedRange=NSMakeRange(start,end-start);
    self.loading=NO;
}
- (void)appendTab:(NSInteger)identity {
    NSTabViewItem *item=[[NSTabViewItem alloc]initWithIdentifier:@(identity)];
    item.label=@"Untitled";item.view=[[NSView alloc]initWithFrame:NSZeroRect];
    [self.tabs addTabViewItem:item];
}
- (void)tabView:(NSTabView *)tabView didSelectTabViewItem:(NSTabViewItem *)item {
    if (self.loading) return;
    NSInteger index=[tabView indexOfTabViewItem:item];
    if(index<0||index>=self.documents.count)return;
    [self storeDocument];self.current=index;[self loadDocument];
}
- (void)createDocument {
    self.creating=NO;
    if(self.documents.count>=16)return;
    [self storeDocument];self.loading=YES;
    NSInteger identity=self.nextID++;
    [self.documents addObject:[@{@"id":@(identity),@"text":@"",@"start":@0,@"end":@0} mutableCopy]];
    [self appendTab:identity];self.current=self.documents.count-1;
    [self.tabs selectTabViewItemAtIndex:self.current];[self loadDocument];
    [self.window makeFirstResponder:self.editor];
}
- (void)newDocument:(id)sender {
    if(self.creating)return;
    self.creating=YES;
    if(self.delayMS>0)dispatch_after(dispatch_time(DISPATCH_TIME_NOW,self.delayMS*NSEC_PER_MSEC),dispatch_get_main_queue(),^{[self createDocument];});
    else [self createDocument];
}
- (BOOL)textView:(NSTextView *)textView shouldChangeTextInRange:(NSRange)range replacementString:(NSString *)string {
    if(self.loading||self.partialBudget<0||!string)return YES;
    if(string.length<=(NSUInteger)self.partialBudget){self.partialBudget-=(NSInteger)string.length;return YES;}
    NSUInteger allowed=(NSUInteger)self.partialBudget;self.partialBudget=0;
    // Faults never split a surrogate or composed sequence themselves.
    if(allowed>0&&allowed<string.length){NSRange cluster=[string rangeOfComposedCharacterSequenceAtIndex:allowed];if(cluster.location<allowed)allowed=cluster.location;}
    if(allowed>0){self.loading=YES;[textView.textStorage replaceCharactersInRange:range withString:[string substringToIndex:allowed]];self.loading=NO;}
    [self.window makeFirstResponder:self.focusSink];return NO;
}
- (void)respond:(NSString *)kind {
    [self storeDocument];
    NSDictionary *state=@{@"version":@1,@"kind":kind,@"pid":@(getpid()),@"editor":@"shared-editor",@"selection_unit":@"utf16",@"selected":@(self.current),@"documents":self.documents};
    NSData *data=[NSJSONSerialization dataWithJSONObject:state options:0 error:nil];
    if(data){fwrite(data.bytes,1,data.length,stdout);fputc('\n',stdout);fflush(stdout);}
}
- (BOOL)integer:(id)value maximum:(NSInteger)maximum into:(NSInteger *)output {
    if(![value isKindOfClass:[NSNumber class]]||CFGetTypeID((__bridge CFTypeRef)value)==CFBooleanGetTypeID())return NO;
    double number=[value doubleValue];NSInteger integer=[value integerValue];
    if(number!=(double)integer||integer<0||integer>maximum)return NO;*output=integer;return YES;
}
- (void)request:(NSData *)line {
    if(line.length>20000)return;
    id parsed=[NSJSONSerialization JSONObjectWithData:line options:0 error:nil];
    if(![parsed isKindOfClass:[NSDictionary class]])return;
    NSDictionary *request=parsed;NSString *kind=request[@"kind"];if(![kind isKindOfClass:[NSString class]])return;
    NSInteger number,start,end;
    if([kind isEqual:@"state"]){}
    else if([kind isEqual:@"new"])[self newDocument:nil];
    else if([kind isEqual:@"select"]&&[self integer:request[@"index"] maximum:self.documents.count-1 into:&number])[self.tabs selectTabViewItemAtIndex:number];
    else if([kind isEqual:@"delay"]&&[self integer:request[@"delay_ms"] maximum:2000 into:&number])self.delayMS=number;
    else if([kind isEqual:@"selection"]&&[self integer:request[@"start"] maximum:self.editor.string.length into:&start]&&[self integer:request[@"end"] maximum:self.editor.string.length into:&end]&&start<=end)self.editor.selectedRange=NSMakeRange(start,end-start);
    else if([kind isEqual:@"text"]){
        NSString *text=request[@"text"];if(![text isKindOfClass:[NSString class]]||[text lengthOfBytesUsingEncoding:NSUTF8StringEncoding]>16000)return;
        [self.editor insertText:text replacementRange:self.editor.selectedRange];
    }
    else if([kind isEqual:@"partial_input"]&&[self integer:request[@"index"] maximum:16000 into:&number])self.partialBudget=number;
    else if([kind isEqual:@"focus_loss"])[self.window makeFirstResponder:self.focusSink];
    else if([kind isEqual:@"quit"])[NSApp terminate:nil];
    else return;
    [self respond:kind];
}
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    self.documents=[NSMutableArray arrayWithArray:@[
        [@{@"id":@1,@"text":@"Synthetic lifecycle content",@"start":@0,@"end":@0} mutableCopy],
        [@{@"id":@2,@"text":@"Untouched — café 日本語 12345",@"start":@0,@"end":@0} mutableCopy]]];
    self.current=0;self.nextID=3;self.partialBudget=-1;self.pending=[NSMutableData data];self.loading=YES;
    self.window=[[NSWindow alloc]initWithContentRect:NSMakeRect(180,180,680,380) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];
    self.window.title=@"OffGrid disposable document fixture";
    self.tabs=[[NSTabView alloc]initWithFrame:NSMakeRect(16,318,648,44)];
    self.tabs.tabViewType=NSTopTabsBezelBorder;[self appendTab:1];[self appendTab:2];self.tabs.delegate=self;
    [self.window.contentView addSubview:self.tabs];
    self.editor=[[NSTextView alloc]initWithFrame:NSMakeRect(16,80,648,228)];
    self.editor.richText=NO;self.editor.automaticQuoteSubstitutionEnabled=NO;self.editor.automaticDashSubstitutionEnabled=NO;
    self.editor.automaticTextReplacementEnabled=NO;self.editor.automaticSpellingCorrectionEnabled=NO;self.editor.delegate=self;
    [self.window.contentView addSubview:self.editor];[self loadDocument];
    NSButton *button=[NSButton buttonWithTitle:@"New blank document" target:self action:@selector(newDocument:)];button.frame=NSMakeRect(16,24,220,32);[self.window.contentView addSubview:button];
    self.focusSink=[[NSTextField alloc]initWithFrame:NSMakeRect(260,24,390,32)];self.focusSink.stringValue=@"Owned focus-loss target";[self.window.contentView addSubview:self.focusSink];
    [self.window makeKeyAndOrderFront:nil];[NSApp activateIgnoringOtherApps:YES];[self.window makeFirstResponder:self.editor];[self respond:@"ready"];
    [NSFileHandle fileHandleWithStandardInput].readabilityHandler=^(NSFileHandle *handle){
        NSData *data=[handle availableData];dispatch_async(dispatch_get_main_queue(),^{
            if(data.length==0){[NSApp terminate:nil];return;}
            if(self.pending.length+data.length>40000){[NSApp terminate:nil];return;}
            [self.pending appendData:data];
            while(YES){
                const unsigned char *bytes=self.pending.bytes;NSUInteger end=0;
                while(end<self.pending.length&&bytes[end]!='\n')end++;
                if(end==self.pending.length)break;
                NSData *line=[self.pending subdataWithRange:NSMakeRange(0,end)];
                [self.pending replaceBytesInRange:NSMakeRange(0,end+1) withBytes:NULL length:0];[self request:line];
            }
        });
    };
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,120*NSEC_PER_SEC),dispatch_get_main_queue(),^{[NSApp terminate:nil];});
}
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender{return YES;}
@end

int main(void) {
    @autoreleasepool {
        NSApplication *app=[NSApplication sharedApplication];[app setActivationPolicy:NSApplicationActivationPolicyRegular];
        OGDocumentFixture *fixture=[OGDocumentFixture new];app.delegate=fixture;[app run];
    }
    return 0;
}
