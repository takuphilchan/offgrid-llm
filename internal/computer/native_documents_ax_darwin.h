// Included by the owned AX bridge only. No public entry point or listener.
// Attribute support is checked on every call; missing support is not emulated.
static NSMutableDictionary *documentObjects;
static NSString *documentKey(AXUIElementRef element,NSMutableDictionary *next) {
    for(NSString *key in next)if(CFEqual(((OGAXReference *)next[key])->element,element))return key;
    for(NSString *key in documentObjects)if(CFEqual(((OGAXReference *)documentObjects[key])->element,element)){
        next[key]=documentObjects[key];return key;
    }
    NSString *key=identifier();next[key]=[[[OGAXReference alloc]initWithElement:element]autorelease];return key;
}
static BOOL documentSettable(AXUIElementRef element,CFStringRef name) {
    Boolean value=false;return AXUIElementIsAttributeSettable(element,name,&value)==kAXErrorSuccess&&value;
}
static BOOL documentPressable(AXUIElementRef element) {
    CFArrayRef names=NULL;BOOL result=NO;
    if(AXUIElementCopyActionNames(element,&names)==kAXErrorSuccess&&names){result=[(NSArray *)names containsObject:(NSString *)kAXPressAction];CFRelease(names);}return result;
}
static NSString *documentWalk(AXUIElementRef element,NSString *parent,NSUInteger depth,NSMutableDictionary *next,NSMutableArray *nodes) {
    if(depth>=64||nodes.count>=1024)return @"computer_document_incomplete";
    pid_t pid=0;if(AXUIElementGetPid(element,&pid)!=kAXErrorSuccess||pid!=[selected[@"pid"] intValue])return @"computer_scope_violation";
    NSString *key=documentKey(element,next);for(NSDictionary *node in nodes)if([node[@"key"] isEqual:key])return @"computer_document_changed";
    NSString *role=stringAttribute(element,kAXRoleAttribute),*subrole=stringAttribute(element,kAXSubroleAttribute);
    BOOL protected=[role isEqual:@"AXSecureTextField"]||[subrole isEqual:@"AXSecureTextField"];
    NSMutableDictionary *node=[@{@"key":key,@"parent":parent?:@"",@"visible":@(![attribute(element,CFSTR("AXHidden")) boolValue]),@"protected":@(protected),@"text":@(!protected&&([role isEqual:(NSString *)kAXTextAreaRole]||[role isEqual:(NSString *)kAXTextFieldRole]))} mutableCopy];
    [nodes addObject:node];[node release];if(protected)return nil;
    id children=attribute(element,kAXChildrenAttribute);
    if(children&&![children isKindOfClass:[NSArray class]])return @"computer_document_incomplete";
    if([(NSArray *)children count]>1024-nodes.count)return @"computer_document_incomplete";
    for(id child in children){if(CFGetTypeID((CFTypeRef)child)!=AXUIElementGetTypeID())return @"computer_document_incomplete";NSString *error=documentWalk((AXUIElementRef)child,key,depth+1,next,nodes);if(error)return error;}
    return nil;
}
static NSString *documentExistingKey(id value,NSDictionary *objects) {
    if(!value||CFGetTypeID((CFTypeRef)value)!=AXUIElementGetTypeID())return nil;
    for(NSString *key in objects)if(CFEqual(((OGAXReference *)objects[key])->element,(CFTypeRef)value))return key;return nil;
}
static NSDictionary *documentTree(void) {
    NSMutableDictionary *next=[NSMutableDictionary dictionary];NSMutableArray *nodes=[NSMutableArray array];
    OGAXReference *window=selected[@"window"];
    NSString *error=documentWalk(window->element,@"",0,next,nodes);if(error){[documentObjects removeAllObjects];return failure(error);}
    NSMutableDictionary *byID=[NSMutableDictionary dictionary];for(NSMutableDictionary *node in nodes)byID[node[@"key"]]=node;
    for(NSString *key in next){
        AXUIElementRef element=((OGAXReference *)next[key])->element;
        if(![stringAttribute(element,kAXRoleAttribute) isEqual:(NSString *)kAXTabGroupRole])continue;
        id tabs=attribute(element,kAXTabsAttribute);if(![tabs isKindOfClass:[NSArray class]]||[tabs count]>256)return failure(@"computer_document_incomplete");
        for(id tab in tabs){
            NSString *tabKey=documentExistingKey(tab,next);NSMutableDictionary *node=byID[tabKey];if(!node)return failure(@"computer_document_changed");
            id value=attribute((AXUIElementRef)tab,kAXValueAttribute);if(![value isKindOfClass:[NSNumber class]]||([value intValue]!=0&&[value intValue]!=1))return failure(@"computer_document_unavailable");
            NSString *title=stringAttribute((AXUIElementRef)tab,kAXTitleAttribute);if([title lengthOfBytesUsingEncoding:NSUTF8StringEncoding]>1024)return failure(@"computer_document_incomplete");
            node[@"tab"]=@YES;node[@"selected"]=@([value boolValue]);node[@"group"]=key;node[@"title"]=title;
            node[@"selectable"]=@(documentPressable((AXUIElementRef)tab)&&[attribute((AXUIElementRef)tab,kAXEnabledAttribute) boolValue]);
            // AXContents explicitly describes the group's selected contents.
            // Missing relationships remain missing, never assigned by focus.
            if([value boolValue]){
                id contents=attribute(element,kAXContentsAttribute);NSMutableArray *links=[NSMutableArray array];
                if(contents&&![contents isKindOfClass:[NSArray class]])return failure(@"computer_document_unavailable");
                if([contents count]>16)return failure(@"computer_document_incomplete");
                for(id content in contents){NSString *link=documentExistingKey(content,next);if(!link)return failure(@"computer_document_incomplete");[links addObject:link];}node[@"controls"]=links;
            }
        }
    }
    [documentObjects release];documentObjects=[next mutableCopy];return success(nodes);
}
static BOOL documentRange(id raw,CFRange *range) {
    return raw&&CFGetTypeID((CFTypeRef)raw)==AXValueGetTypeID()&&AXValueGetType((AXValueRef)raw)==kAXValueCFRangeType&&AXValueGetValue((AXValueRef)raw,kAXValueCFRangeType,range)&&range->location>=0&&range->length>=0;
}
static NSString *documentComposition(AXUIElementRef element) {
    // A typed provider extension is usable only when explicitly supported. An
    // unsupported attribute must not be treated like a zero-length marked range.
    CFArrayRef names=NULL;if(AXUIElementCopyAttributeNames(element,&names)!=kAXErrorSuccess||!names)return @"unknown";
    BOOL supported=[(NSArray *)names containsObject:@"AXTextInputMarkedRange"];CFRelease(names);if(!supported)return @"unknown";
    CFTypeRef value=NULL;AXError e=AXUIElementCopyAttributeValue(element,CFSTR("AXTextInputMarkedRange"),&value);
    if(e==kAXErrorNoValue)return @"stable";
    CFRange range;BOOL valid=e==kAXErrorSuccess&&documentRange((id)value,&range);if(value)CFRelease(value);
    return valid?(range.length>0?@"active":@"stable"):@"unknown";
}
static NSDictionary *documentText(AXUIElementRef element) {
    if(!withinTarget(element))return failure(@"computer_scope_violation");
    NSString *role=stringAttribute(element,kAXRoleAttribute),*subrole=stringAttribute(element,kAXSubroleAttribute);
    if([subrole isEqual:@"AXSecureTextField"]||(![role isEqual:(NSString *)kAXTextAreaRole]&&![role isEqual:(NSString *)kAXTextFieldRole]))return failure(@"computer_document_unavailable");
    id text=attribute(element,kAXValueAttribute),selectedText=attribute(element,kAXSelectedTextAttribute);CFRange selection;
    if(![text isKindOfClass:[NSString class]]||[text lengthOfBytesUsingEncoding:NSUTF8StringEncoding]>65536||![selectedText isKindOfClass:[NSString class]])return failure(@"computer_document_incomplete");
    if(!documentRange(attribute(element,kAXSelectedTextRangeAttribute),&selection)||selection.location>(CFIndex)[text length]||selection.length>(CFIndex)[text length]-selection.location)return failure(@"computer_document_range_invalid");
    if(![[text substringWithRange:NSMakeRange(selection.location,selection.length)] isEqual:selectedText])return failure(@"computer_document_changed");
    return success(@{@"text":text,@"selected_text":selectedText,@"selection":@{@"start":@(selection.location),@"end":@(selection.location+selection.length)},@"composition":documentComposition(element),@"focused":@([attribute(element,kAXFocusedAttribute) boolValue]),@"select_text":@(documentSettable(element,kAXSelectedTextRangeAttribute)),@"replace_selection":@(documentSettable(element,kAXSelectedTextAttribute)),@"replace_value":@(documentSettable(element,kAXValueAttribute))});
}
static NSDictionary *documentRequest(NSString *kind,NSDictionary *args) {
    if([kind isEqual:@"document_tree"])return documentTree();
    NSString *key=args[[kind isEqual:@"document_select"]?@"document":@"editor"];OGAXReference *reference=documentObjects[key];
    if(!reference||!withinTarget(reference->element))return failure(@"computer_document_changed");
    if([kind isEqual:@"document_text"])return documentText(reference->element);
    if(!targetValid(YES))return failure(@"computer_scope_violation");
    if([kind isEqual:@"document_select"]){
        // Rebuild membership immediately before dispatch; only an exact tab in
        // the selected window is eligible for its semantic Press action.
        NSDictionary *tree=documentTree();if(tree[@"error"])return tree;BOOL found=NO;
        for(NSDictionary *node in tree[@"result"])if([node[@"key"] isEqual:key]&&[node[@"tab"] boolValue]&&[node[@"selectable"] boolValue])found=YES;
        reference=documentObjects[key];if(!found||!reference||!targetValid(YES))return failure(@"computer_document_changed");
        return AXUIElementPerformAction(reference->element,kAXPressAction)==kAXErrorSuccess?success(@YES):failure(@"computer_uncertain_outcome");
    }
    if(![kind isEqual:@"document_edit"])return failure(@"computer_invalid_action");
    NSDictionary *tree=documentTree();if(tree[@"error"])return tree;
    NSMutableDictionary *byID=[NSMutableDictionary dictionary];NSDictionary *active=nil;
    for(NSDictionary *node in tree[@"result"]){byID[node[@"key"]]=node;if([node[@"tab"] boolValue]&&[node[@"selected"] boolValue]){if(active)return failure(@"computer_document_changed");active=node;}}
    if(!active||![active[@"key"] isEqual:args[@"document"]])return failure(@"computer_document_changed");
    NSString *ancestor=key;BOOL related=NO;
    for(int depth=0;ancestor.length&&depth<64;depth++){
        if([ancestor isEqual:active[@"key"]]||[active[@"controls"] containsObject:ancestor])related=YES;
        ancestor=byID[ancestor][@"parent"];
    }
    reference=documentObjects[key];if(!related||!reference)return failure(@"computer_document_changed");
    NSDictionary *read=documentText(reference->element);if(read[@"error"])return read;NSDictionary *text=read[@"result"];
    if(![text[@"composition"] isEqual:@"stable"])return failure(@"computer_document_composition_unstable");
    if(![text[@"focused"] boolValue]||![text[@"text"] isEqual:args[@"preimage"]]||![text[@"selection"] isEqual:args[@"selection"]])return failure(@"computer_document_changed");
    NSString *operation=args[@"kind"];CFStringRef attributeName=NULL;CFTypeRef value=NULL;BOOL releaseValue=NO;
    if([operation isEqual:@"select_text"]){
        NSInteger start=[args[@"range"][@"start"] integerValue],end=[args[@"range"][@"end"] integerValue];
        if(start<0||end<start||end>[text[@"text"] length])return failure(@"computer_document_range_invalid");
        CFRange range=CFRangeMake(start,end-start);value=AXValueCreate(kAXValueCFRangeType,&range);releaseValue=YES;attributeName=kAXSelectedTextRangeAttribute;
    }else if([operation isEqual:@"insert_text"]||[operation isEqual:@"replace_selection"]){attributeName=kAXSelectedTextAttribute;value=(CFTypeRef)args[@"text"];
    }else if([operation isEqual:@"replace_value"]){attributeName=kAXValueAttribute;value=(CFTypeRef)args[@"text"];
    }else return failure(@"computer_invalid_action");
    if(!value||!documentSettable(reference->element,attributeName)){if(releaseValue&&value)CFRelease(value);return failure(@"computer_document_unavailable");}
    if(!targetValid(YES)||![attribute(reference->element,kAXFocusedAttribute) boolValue]){if(releaseValue)CFRelease(value);return failure(@"computer_scope_violation");}
    AXError result=AXUIElementSetAttributeValue(reference->element,attributeName,value);if(releaseValue)CFRelease(value);
    return result==kAXErrorSuccess?success(@YES):failure(@"computer_uncertain_outcome");
}
