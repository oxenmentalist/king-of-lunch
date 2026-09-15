#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>
#import "native.h"

extern void goNativeEvent(char *, uint64_t, char *, uint64_t);
static void emit(NSString *kind, uint64_t identifier, NSString *value, uint64_t token) {
    goNativeEvent((char *)kind.UTF8String, identifier, (char *)(value ?: @"").UTF8String, token);
}
static NSString *copied(const char *text) { return text ? [NSString stringWithUTF8String:text] : @""; }
static NSString *documentBase = @"kol-document://viewer/";
static WKWebsiteDataStore *documentDataStore;
static NSString *controlScript;
static WKContentWorld *world(void) { return [WKContentWorld worldWithName:@"KING OF LUNCH controls"]; }
static NSString *json(id object) {
    NSData *data = [NSJSONSerialization dataWithJSONObject:object ?: [NSNull null] options:NSJSONWritingFragmentsAllowed error:nil];
    return data ? [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] : @"null";
}
// Lazy images below the viewport would otherwise print as empty boxes.
static NSString *printPreparationScript = @"await Promise.all(Array.from(document.images, image => { image.loading = 'eager'; return image.decode().catch(() => {}); }))";
static NSString *anchorScript(NSString *fragment) {
    return [NSString stringWithFormat:@"(() => { const id=%@; if (!id) { window.scrollTo(0,0); return; } const target=document.getElementById(id); if (target) target.scrollIntoView(); })()", json(fragment)];
}

@interface KOLDocument : NSWindowController <NSWindowDelegate, WKNavigationDelegate>
@property uint64_t identifier;
@property uint64_t generation;
@property uint64_t navigationGeneration;
@property int fontSize;
@property BOOL hasContent;
@property BOOL closed;
@property (copy) NSString *path;
@property (copy) NSString *pendingFragment;
@property (strong) WKWebView *web;
@property (strong) WKNavigation *navigation;
@property double scrollX;
@property double scrollY;
@property CFAbsoluteTime started;
@property (strong) NSAlert *alert;
- (void)loadHTML:(NSString *)html generation:(uint64_t)generation;
- (void)applyZoom;
- (void)navigateToFragment:(NSString *)fragment;
- (void)printToURL:(NSURL *)destination;
@end

@interface KOLDelegate : NSObject <NSApplicationDelegate, NSMenuItemValidation>
@property (strong) NSMutableDictionary<NSNumber *, KOLDocument *> *documents;
@property BOOL receivedOpen;
- (void)openDocument:(id)sender;
- (void)reloadDocument:(id)sender;
- (void)zoomIn:(id)sender;
- (void)zoomOut:(id)sender;
- (void)zoomReset:(id)sender;
- (void)printDocument:(id)sender;
@end
static KOLDelegate *delegate;

@implementation KOLDocument
- (void)dealloc { emit(@"disposed", self.identifier, @"", 0); }
- (instancetype)init {
    NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 880, 740)
        styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
        backing:NSBackingStoreBuffered defer:NO];
    self = [super initWithWindow:window];
    if (self) {
        self.fontSize = 16;
        window.delegate = self;
        window.releasedWhenClosed = NO;
        window.minSize = NSMakeSize(360, 240);
        window.backgroundColor = [NSColor colorWithSRGBRed:31.0/255 green:31.0/255 blue:40.0/255 alpha:1];
        window.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
        config.websiteDataStore = documentDataStore;
        config.defaultWebpagePreferences.allowsContentJavaScript = NO;
        config.preferences.javaScriptCanOpenWindowsAutomatically = NO;
        // Code blocks and table headings keep their light tint on paper.
        if (@available(macOS 13.3, *)) config.preferences.shouldPrintBackgrounds = YES;
        WKUserScript *controls = [[WKUserScript alloc] initWithSource:controlScript injectionTime:WKUserScriptInjectionTimeAtDocumentEnd forMainFrameOnly:YES inContentWorld:world()];
        [config.userContentController addUserScript:controls];
        self.web = [[WKWebView alloc] initWithFrame:window.contentView.bounds configuration:config];
        self.web.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        self.web.navigationDelegate = self;
        self.web.allowsBackForwardNavigationGestures = NO;
        self.web.underPageBackgroundColor = window.backgroundColor;
        [window.contentView addSubview:self.web];
        [window center];
        [window setFrameAutosaveName:@"DocumentWindow"];
    }
    return self;
}
- (void)windowWillClose:(NSNotification *)notification {
    self.closed = YES;
    [self.web stopLoading];
    self.web.navigationDelegate = nil;
    uint64_t identifier = self.identifier;
    emit(@"closed", identifier, @"", 0);
    [delegate.documents removeObjectForKey:@(identifier)];
}
- (void)applyZoom {
    NSString *script = [NSString stringWithFormat:@"document.documentElement.style.fontSize='%dpx'", self.fontSize];
    [self.web evaluateJavaScript:script inFrame:nil inContentWorld:world() completionHandler:nil];
}
- (void)navigateToFragment:(NSString *)fragment {
    if (self.closed) return;
    if (self.hasContent && !self.web.loading && self.navigationGeneration == self.generation) {
        [self.web evaluateJavaScript:anchorScript(fragment) inFrame:nil inContentWorld:world() completionHandler:nil];
    } else {
        self.pendingFragment = fragment;
    }
}
- (void)loadHTML:(NSString *)html generation:(uint64_t)generation {
    if (self.closed || self.generation != generation) return;
    void (^load)(void) = ^{
        if (self.closed || self.generation != generation) return;
        // Images are embedded by Go. No base URL grants filesystem access.
        self.navigationGeneration = generation;
        self.navigation = [self.web loadHTMLString:html baseURL:[NSURL URLWithString:documentBase]];
    };
    if (self.hasContent) {
        [self.web evaluateJavaScript:@"[window.scrollX,window.scrollY]" inFrame:nil inContentWorld:world()
            completionHandler:^(id value, NSError *error) {
                if ([value isKindOfClass:[NSArray class]] && [value count] == 2) {
                    self.scrollX = [value[0] doubleValue]; self.scrollY = [value[1] doubleValue];
                }
                load();
            }];
    } else { load(); }
}
// destination nil shows the print panel; a file URL saves a PDF without UI.
- (void)printToURL:(NSURL *)destination {
    if (self.closed || !self.hasContent || self.window.attachedSheet) {
        if (destination) emit(@"printed", self.identifier, @"false", 0);
        return;
    }
    [self.web callAsyncJavaScript:printPreparationScript arguments:nil inFrame:nil inContentWorld:world() completionHandler:^(id value, NSError *error) {
        if (self.closed || self.window.attachedSheet) {
            if (destination) emit(@"printed", self.identifier, @"false", 0);
            return;
        }
        NSPrintInfo *info = [[NSPrintInfo sharedPrintInfo] copy];
        info.horizontalPagination = NSPrintingPaginationModeFit;
        info.verticalPagination = NSPrintingPaginationModeAutomatic;
        info.horizontallyCentered = NO;
        info.verticallyCentered = NO;
        info.leftMargin = info.rightMargin = 54;
        info.topMargin = info.bottomMargin = 54;
        info.dictionary[NSPrintHeaderAndFooter] = @YES;
        if (destination) {
            info.jobDisposition = NSPrintSaveJob;
            info.dictionary[NSPrintJobSavingURL] = destination;
        }
        NSPrintOperation *operation = [self.web printOperationWithPrintInfo:info];
        operation.jobTitle = self.path.lastPathComponent.stringByDeletingPathExtension;
        operation.showsPrintPanel = destination == nil;
        operation.showsProgressPanel = destination == nil;
        operation.printPanel.options |= NSPrintPanelShowsPaperSize | NSPrintPanelShowsOrientation | NSPrintPanelShowsScaling;
        // WKWebView's printing view is laid out from this frame; a zero frame prints blank pages.
        operation.view.frame = self.web.bounds;
        [operation runOperationModalForWindow:self.window delegate:self didRunSelector:@selector(printOperationDidRun:success:contextInfo:) contextInfo:destination ? (void *)1 : NULL];
    }];
}
- (void)printOperationDidRun:(NSPrintOperation *)operation success:(BOOL)success contextInfo:(void *)contextInfo {
    if (contextInfo) emit(@"printed", self.identifier, success ? @"true" : @"false", 0);
}
- (void)webView:(WKWebView *)web didFinishNavigation:(WKNavigation *)navigation {
    if (self.closed || navigation != self.navigation || self.navigationGeneration != self.generation) return;
    self.hasContent = YES;
    uint64_t generation = self.generation;
    NSString *script = [NSString stringWithFormat:@"document.documentElement.style.fontSize='%dpx'; window.scrollTo(%f,%f); %@; true", self.fontSize, self.scrollX, self.scrollY, self.pendingFragment ? anchorScript(self.pendingFragment) : @""];
    self.pendingFragment = nil;
    [web evaluateJavaScript:script inFrame:nil inContentWorld:world() completionHandler:^(id value, NSError *error) {
        if (self.closed || generation != self.generation) return;
        if (error) { emit(@"error", self.identifier, error.localizedDescription, generation); return; }
        emit(@"loaded", self.identifier, json(@{@"milliseconds": @((CFAbsoluteTimeGetCurrent()-self.started)*1000), @"generation": @(generation)}), generation);
    }];
}
- (void)webView:(WKWebView *)web didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    if (error.code != NSURLErrorCancelled && navigation == self.navigation) kol_error(self.identifier, self.generation, error.localizedDescription.UTF8String);
}
- (void)webView:(WKWebView *)web didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    [self webView:web didFailNavigation:navigation withError:error];
}
- (void)webViewWebContentProcessDidTerminate:(WKWebView *)web {
    kol_error(self.identifier, self.generation, "The document renderer stopped. Press Command-R to reload the file.");
}
- (void)webView:(WKWebView *)web decidePolicyForNavigationAction:(WKNavigationAction *)action decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    NSURL *url = action.request.URL;
    if (action.navigationType == WKNavigationTypeOther && ([url.absoluteString isEqualToString:@"about:blank"] || [url.absoluteString isEqualToString:documentBase])) {
        decisionHandler(WKNavigationActionPolicyAllow); return;
    }
    if (action.navigationType == WKNavigationTypeLinkActivated) {
        NSString *scheme = url.scheme.lowercaseString;
        if ([scheme isEqualToString:@"kol-document"] && [url.host isEqualToString:@"viewer"]) {
            NSString *fragment = url.fragment.stringByRemovingPercentEncoding ?: @"";
            [self navigateToFragment:fragment];
            decisionHandler(WKNavigationActionPolicyCancel); return;
        }
        if ([scheme isEqualToString:@"https"] || [scheme isEqualToString:@"http"]) [[NSWorkspace sharedWorkspace] openURL:url];
        else if ([scheme isEqualToString:@"kol-file"] && [url.host isEqualToString:@"document"]) {
            NSString *ext = url.pathExtension.lowercaseString;
            if ([ext isEqualToString:@"md"] || [ext isEqualToString:@"markdown"]) {
                NSString *fragment = url.fragment.stringByRemovingPercentEncoding;
                emit(@"open-link", 0, json(@{@"path": url.path, @"fragment": fragment ?: [NSNull null]}), 0);
            }
        }
    }
    decisionHandler(WKNavigationActionPolicyCancel);
}
@end

static KOLDocument *focused(void) {
    for (KOLDocument *document in delegate.documents.allValues) if (document.window == NSApp.keyWindow) return document;
    return nil;
}
static KOLDocument *actionDocument(id sender) {
    return [sender isKindOfClass:[KOLDocument class]] ? sender : focused();
}
@implementation KOLDelegate
- (instancetype)init { self = [super init]; if (self) self.documents = [NSMutableDictionary dictionary]; return self; }
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    emit(@"ready", 0, @"", 0);
    [NSApp activateIgnoringOtherApps:YES];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 200*NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
        if (!self.receivedOpen && self.documents.count == 0) [self openDocument:nil];
    });
}
- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
    self.receivedOpen = YES;
    for (NSURL *url in urls) if (url.isFileURL) emit(@"open", 0, url.path, 0);
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)application hasVisibleWindows:(BOOL)visible {
    if (!visible) [self openDocument:nil];
    return YES;
}
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)application { return NO; }
- (void)openDocument:(id)sender {
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    panel.canChooseDirectories = NO; panel.allowsMultipleSelection = YES;
    panel.message = @"Choose a Markdown file to read.";
    [panel beginWithCompletionHandler:^(NSModalResponse result) {
        if (result == NSModalResponseOK) for (NSURL *url in panel.URLs) emit(@"open", 0, url.path, 0);
    }];
}
- (void)reloadDocument:(id)sender { KOLDocument *d = actionDocument(sender); if (d) emit(@"reload", d.identifier, @"", 0); }
- (void)zoomIn:(id)sender { KOLDocument *d = actionDocument(sender); if (d) emit(@"zoom", d.identifier, @"2", 0); }
- (void)zoomOut:(id)sender { KOLDocument *d = actionDocument(sender); if (d) emit(@"zoom", d.identifier, @"-2", 0); }
- (void)zoomReset:(id)sender { KOLDocument *d = actionDocument(sender); if (d) emit(@"zoom", d.identifier, @"0", 0); }
- (void)printDocument:(id)sender { [actionDocument(sender) printToURL:nil]; }
- (BOOL)validateMenuItem:(NSMenuItem *)item {
    KOLDocument *d = focused();
    if (item.action == @selector(reloadDocument:) || item.action == @selector(zoomReset:)) return d != nil;
    if (item.action == @selector(printDocument:)) return d && d.hasContent && !d.window.attachedSheet;
    if (item.action == @selector(zoomIn:)) return d && d.fontSize < 32;
    if (item.action == @selector(zoomOut:)) return d && d.fontSize > 10;
    return YES;
}
@end

static NSMenu *submenu(NSMenu *bar, NSString *title) {
    NSMenuItem *parent = [bar addItemWithTitle:title action:nil keyEquivalent:@""];
    NSMenu *menu = [[NSMenu alloc] initWithTitle:title]; parent.submenu = menu; return menu;
}
static NSMenuItem *item(NSMenu *menu, NSString *title, SEL action, NSString *key, id target) {
    NSMenuItem *i = [menu addItemWithTitle:title action:action keyEquivalent:key]; i.target = target; return i;
}
void kol_run(const char *controls) {
    @autoreleasepool {
        controlScript = copied(controls);
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        documentDataStore = [WKWebsiteDataStore nonPersistentDataStore];
        delegate = [[KOLDelegate alloc] init]; NSApp.delegate = delegate;
        NSMenu *bar = [[NSMenu alloc] init];
        NSMenu *app = submenu(bar, @"KING OF LUNCH");
        item(app, @"About KING OF LUNCH", @selector(orderFrontStandardAboutPanel:), @"", NSApp);
        [app addItem:[NSMenuItem separatorItem]];
        item(app, @"Hide KING OF LUNCH", @selector(hide:), @"h", NSApp);
        NSMenuItem *hideOthers = item(app, @"Hide Others", @selector(hideOtherApplications:), @"h", NSApp);
        hideOthers.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagOption;
        item(app, @"Show All", @selector(unhideAllApplications:), @"", NSApp);
        [app addItem:[NSMenuItem separatorItem]];
        item(app, @"Quit KING OF LUNCH", @selector(terminate:), @"q", NSApp);
        NSMenu *file = submenu(bar, @"File");
        item(file, @"Open…", @selector(openDocument:), @"o", delegate);
        item(file, @"Reload", @selector(reloadDocument:), @"r", delegate);
        [file addItem:[NSMenuItem separatorItem]];
        NSMenuItem *pageSetup = item(file, @"Page Setup…", @selector(runPageLayout:), @"p", NSApp);
        pageSetup.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
        item(file, @"Print…", @selector(printDocument:), @"p", delegate);
        [file addItem:[NSMenuItem separatorItem]];
        item(file, @"Close Window", @selector(performClose:), @"w", nil);
        NSMenu *edit = submenu(bar, @"Edit");
        item(edit, @"Cut", @selector(cut:), @"x", nil);
        item(edit, @"Copy", @selector(copy:), @"c", nil);
        item(edit, @"Paste", @selector(paste:), @"v", nil);
        item(edit, @"Select All", @selector(selectAll:), @"a", nil);
        NSMenu *view = submenu(bar, @"View");
        item(view, @"Zoom In", @selector(zoomIn:), @"+", delegate);
        NSMenuItem *alternate = item(view, @"Zoom In", @selector(zoomIn:), @"=", delegate);
        alternate.hidden = YES; alternate.allowsKeyEquivalentWhenHidden = YES;
        item(view, @"Zoom Out", @selector(zoomOut:), @"-", delegate);
        item(view, @"Actual Size", @selector(zoomReset:), @"0", delegate);
        NSMenu *window = submenu(bar, @"Window");
        item(window, @"Minimize", @selector(performMiniaturize:), @"m", nil);
        item(window, @"Zoom", @selector(performZoom:), @"", nil);
        NSApp.windowsMenu = window; NSApp.mainMenu = bar;
        [NSApp run];
        for (KOLDocument *d in delegate.documents.allValues.copy) [d.window close];
        NSApp.delegate = nil; delegate = nil; documentDataStore = nil; controlScript = nil;
    }
}
void kol_show(uint64_t identifier, const char *path) {
    @autoreleasepool {
    NSString *name = copied(path);
    dispatch_async(dispatch_get_main_queue(), ^{
        delegate.receivedOpen = YES;
        KOLDocument *d = delegate.documents[@(identifier)];
        if (!d) {
            d = [[KOLDocument alloc] init]; d.identifier = identifier; d.path = name;
            d.window.title = name.lastPathComponent; d.window.representedURL = [NSURL fileURLWithPath:name];
            delegate.documents[@(identifier)] = d;
            emit(@"opened", identifier, name, 0);
        }
        [d.window makeKeyAndOrderFront:nil]; [NSApp activateIgnoringOtherApps:YES];
    });
    }
}
void kol_begin(uint64_t identifier, uint64_t generation) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)]; d.generation = generation; d.started = CFAbsoluteTimeGetCurrent();
    });
    }
}
void kol_anchor(uint64_t identifier, const char *fragment) {
    @autoreleasepool {
    NSString *target = copied(fragment);
    dispatch_async(dispatch_get_main_queue(), ^{ [delegate.documents[@(identifier)] navigateToFragment:target]; });
    }
}
void kol_content(uint64_t identifier, uint64_t generation, const char *html) {
    @autoreleasepool {
    NSString *content = copied(html);
    dispatch_async(dispatch_get_main_queue(), ^{ [delegate.documents[@(identifier)] loadHTML:content generation:generation]; });
    }
}
void kol_error(uint64_t identifier, uint64_t generation, const char *message) {
    @autoreleasepool {
    NSString *text = copied(message);
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)];
        if (identifier && (!d || d.generation != generation)) return;
        emit(@"error", identifier, text, generation);
        NSAlert *alert = [[NSAlert alloc] init]; alert.messageText = @"Couldn’t open the document";
        alert.informativeText = d ? [NSString stringWithFormat:@"%@\n\n%@", d.path, text] : text;
        [alert addButtonWithTitle:@"OK"];
        if (d) {
            if (d.alert) [d.window endSheet:d.alert.window];
            d.alert = alert;
            [alert beginSheetModalForWindow:d.window completionHandler:^(NSModalResponse response) { d.alert = nil; }];
        } else { [alert runModal]; }
    });
    }
}
void kol_zoom(uint64_t identifier, int size) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)]; d.fontSize = size; [d applyZoom];
    });
    }
}
char *kol_launch(const char *path, const char *application) {
    @autoreleasepool {
        NSURL *file = [NSURL fileURLWithPath:copied(path)];
        NSString *appPath = copied(application);
        NSURL *app = appPath.length ? [NSURL fileURLWithPath:appPath] : [[NSWorkspace sharedWorkspace] URLForApplicationWithBundleIdentifier:@"dev.kingoflunch.app"];
        if (!app) return strdup("KING OF LUNCH.app was not found. Run make install, or keep kol beside the built app.");
        __block BOOL done = NO; __block NSString *failure = nil;
        NSWorkspaceOpenConfiguration *config = [NSWorkspaceOpenConfiguration configuration];
        config.activates = YES;
        [[NSWorkspace sharedWorkspace] openURLs:@[file] withApplicationAtURL:app configuration:config completionHandler:^(NSRunningApplication *running, NSError *error) {
            dispatch_async(dispatch_get_main_queue(), ^{ failure = error.localizedDescription; done = YES; });
        }];
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:15];
        while (!done && deadline.timeIntervalSinceNow > 0) [[NSRunLoop currentRunLoop] runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
        if (!done) return strdup("Timed out asking macOS to open KING OF LUNCH.");
        return failure ? strdup(failure.UTF8String) : NULL;
    }
}
void kol_evaluate(uint64_t identifier, uint64_t token, const char *script) {
    @autoreleasepool {
    NSString *code = copied(script);
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)];
        if (!d) { emit(@"evaluated", identifier, @"{\"error\":\"window closed\"}", token); return; }
        [d.web evaluateJavaScript:code inFrame:nil inContentWorld:world() completionHandler:^(id value, NSError *error) {
            emit(@"evaluated", identifier, error ? json(@{@"error": error.localizedDescription}) : json(value), token);
        }];
    });
    }
}
void kol_action(uint64_t identifier, const char *action) {
    @autoreleasepool {
    NSString *name = copied(action);
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)]; if (!d) return;
        [d.window makeKeyAndOrderFront:nil];
        if ([name isEqualToString:@"reload"]) [delegate reloadDocument:d];
        else if ([name isEqualToString:@"zoomIn"]) [delegate zoomIn:d];
        else if ([name isEqualToString:@"zoomOut"]) [delegate zoomOut:d];
        else if ([name isEqualToString:@"zoomReset"]) [delegate zoomReset:d];
        else if ([name isEqualToString:@"close"]) [d.window performClose:nil];
        else if ([name isEqualToString:@"dismissError"] && d.alert) [d.window endSheet:d.alert.window returnCode:NSModalResponseOK];
    });
    }
}
void kol_print_pdf(uint64_t identifier, const char *path) {
    @autoreleasepool {
    NSURL *destination = [NSURL fileURLWithPath:copied(path)];
    dispatch_async(dispatch_get_main_queue(), ^{
        KOLDocument *d = delegate.documents[@(identifier)];
        if (d) [d printToURL:destination]; else emit(@"printed", identifier, @"false", 0);
    });
    }
}
void kol_resize(uint64_t identifier, int width, int height) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{ [delegate.documents[@(identifier)].window setContentSize:NSMakeSize(width,height)]; });
    }
}
void kol_stop(void) {
    @autoreleasepool {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:NO];
    });
    }
}
