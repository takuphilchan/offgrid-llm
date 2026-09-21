#include <gtk/gtk.h>
#include <json-glib/json-glib.h>
#include <gio/gio.h>
#include <unistd.h>
#include <arpa/inet.h>
#include <string.h>

// The native GUI owns consent and session-lock revocation, not inference. It
// receives only bounded dialog text over inherited pipes. No command execution.
static GMutex output_lock;
static gboolean stopped=FALSE;
static GtkWidget *dialog=NULL;

static gboolean write_all(const void *data,size_t size){const char *bytes=data;while(size){ssize_t count=write(STDOUT_FILENO,bytes,size);if(count<=0)return FALSE;bytes+=count;size-=count;}return TRUE;}
static void message(const char *kind,const char *id,gboolean allowed){
    JsonBuilder *builder=json_builder_new();json_builder_begin_object(builder);json_builder_set_member_name(builder,"kind");json_builder_add_string_value(builder,kind);
    if(id){json_builder_set_member_name(builder,"id");json_builder_add_string_value(builder,id);json_builder_set_member_name(builder,"allowed");json_builder_add_boolean_value(builder,allowed);}
    json_builder_end_object(builder);JsonNode *root=json_builder_get_root(builder);JsonGenerator *generator=json_generator_new();json_generator_set_root(generator,root);gsize length=0;char *body=json_generator_to_data(generator,&length);
    uint32_t header=htonl((uint32_t)length);g_mutex_lock(&output_lock);write_all(&header,4);write_all(body,length);g_mutex_unlock(&output_lock);g_free(body);json_node_free(root);g_object_unref(generator);g_object_unref(builder);
}
static gboolean stop_control(gpointer ignored){if(stopped)return G_SOURCE_REMOVE;stopped=TRUE;message("stop",NULL,FALSE);if(dialog)gtk_widget_destroy(dialog);gtk_main_quit();return G_SOURCE_REMOVE;}
static void clicked(GtkButton *button,gpointer data){stop_control(NULL);}
static gboolean closing(GtkWidget *widget,GdkEvent *event,gpointer data){stop_control(NULL);return TRUE;}
static gboolean key_press(GtkWidget *widget,GdkEventKey *event,gpointer data){if(event->keyval==GDK_KEY_Escape){stop_control(NULL);return TRUE;}return FALSE;}
static void locked(GDBusConnection *connection,const char *sender,const char *path,const char *interface,const char *signal,GVariant *parameters,gpointer data){gboolean active=TRUE;if(g_variant_is_of_type(parameters,G_VARIANT_TYPE("(b)")))g_variant_get(parameters,"(b)",&active);if(active)stop_control(NULL);}
static void vanished(GDBusConnection *connection,const char *name,gpointer data){stop_control(NULL);}
static gboolean read_all(void *data,size_t size){char *bytes=data;while(size){ssize_t count=read(STDIN_FILENO,bytes,size);if(count<=0)return FALSE;bytes+=count;size-=count;}return TRUE;}
typedef struct {char *id;char *text;} Prompt;
static gboolean confirm(gpointer data){
    Prompt *prompt=data;
    if(!stopped){
        dialog=gtk_dialog_new_with_buttons("OffGrid computer access",NULL,GTK_DIALOG_MODAL,"Deny",GTK_RESPONSE_CANCEL,"Approve",GTK_RESPONSE_ACCEPT,NULL);
        gtk_dialog_set_default_response(GTK_DIALOG(dialog),GTK_RESPONSE_CANCEL);gtk_window_set_default_size(GTK_WINDOW(dialog),600,400);
        GtkWidget *scroll=gtk_scrolled_window_new(NULL,NULL);gtk_widget_set_vexpand(scroll,TRUE);GtkWidget *text=gtk_text_view_new();gtk_text_view_set_editable(GTK_TEXT_VIEW(text),FALSE);gtk_text_view_set_cursor_visible(GTK_TEXT_VIEW(text),FALSE);gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(text),GTK_WRAP_WORD_CHAR);gtk_text_buffer_set_text(gtk_text_view_get_buffer(GTK_TEXT_VIEW(text)),prompt->text,-1);gtk_container_add(GTK_CONTAINER(scroll),text);gtk_container_add(GTK_CONTAINER(gtk_dialog_get_content_area(GTK_DIALOG(dialog))),scroll);gtk_widget_show_all(dialog);
        gint response=gtk_dialog_run(GTK_DIALOG(dialog));if(!stopped){message("consent",prompt->id,response==GTK_RESPONSE_ACCEPT);gtk_widget_destroy(dialog);}dialog=NULL;
    }
    g_free(prompt->id);g_free(prompt->text);g_free(prompt);return G_SOURCE_REMOVE;
}
static gpointer read_requests(gpointer ignored){
    while(TRUE){
        uint32_t length;if(!read_all(&length,4))break;length=ntohl(length);if(!length||length>1048576)break;
        char *body=g_malloc(length+1);if(!read_all(body,length)){g_free(body);break;}body[length]=0;
        JsonParser *parser=json_parser_new();GError *error=NULL;gboolean valid=json_parser_load_from_data(parser,body,length,&error);g_free(body);
        JsonNode *root=valid?json_parser_get_root(parser):NULL;
        if(!root||!JSON_NODE_HOLDS_OBJECT(root)){g_clear_error(&error);g_object_unref(parser);break;}
        JsonObject *object=json_node_get_object(root);const char *keys[]={"kind","id","text"};valid=json_object_get_size(object)==3;
        for(int i=0;i<3&&valid;i++){JsonNode *node=json_object_get_member(object,keys[i]);valid=node&&json_node_get_value_type(node)==G_TYPE_STRING;}
        if(!valid||strcmp(json_object_get_string_member(object,"kind"),"confirm")!=0){g_object_unref(parser);break;}
        const char *id=json_object_get_string_member(object,"id"),*text=json_object_get_string_member(object,"text");if(strlen(id)>128||strlen(text)>200000){g_object_unref(parser);break;}
        Prompt *prompt=g_new0(Prompt,1);prompt->id=g_strdup(id);prompt->text=g_strdup(text);g_object_unref(parser);g_main_context_invoke(NULL,confirm,prompt);
    }
    g_main_context_invoke(NULL,stop_control,NULL);return NULL;
}
int main(int argc,char **argv){
    if(!gtk_init_check(&argc,&argv))return 1;
    GError *error=NULL;GDBusConnection *bus=g_bus_get_sync(G_BUS_TYPE_SESSION,NULL,&error);if(!bus){g_clear_error(&error);return 1;}
    const char *names[]={"org.gnome.ScreenSaver","org.freedesktop.ScreenSaver","org.freedesktop.ScreenSaver"};
    const char *paths[]={"/org/gnome/ScreenSaver","/ScreenSaver","/org/freedesktop/ScreenSaver"};
    int selected=-1;
    for(int i=0;i<3;i++){
        GVariant *reply=g_dbus_connection_call_sync(bus,names[i],paths[i],names[i],"GetActive",NULL,G_VARIANT_TYPE("(b)"),G_DBUS_CALL_FLAGS_NONE,2000,NULL,&error);
        if(reply){gboolean active=TRUE;g_variant_get(reply,"(b)",&active);g_variant_unref(reply);if(active){g_object_unref(bus);return 1;}selected=i;break;}g_clear_error(&error);
    }
    // No assumed equivalence across desktops. If lock state cannot be watched,
    // refuse control instead of treating AT-SPI availability as OS consent.
    if(selected<0){g_object_unref(bus);return 1;}
    guint watch=g_bus_watch_name_on_connection(bus,names[selected],G_BUS_NAME_WATCHER_FLAGS_NONE,NULL,vanished,NULL,NULL);
    guint subscription=g_dbus_connection_signal_subscribe(bus,names[selected],names[selected],"ActiveChanged",paths[selected],NULL,G_DBUS_SIGNAL_FLAGS_NONE,locked,NULL,NULL);
    GtkWidget *window=gtk_window_new(GTK_WINDOW_TOPLEVEL);gtk_window_set_title(GTK_WINDOW(window),"OffGrid computer control");gtk_window_set_default_size(GTK_WINDOW(window),560,150);gtk_window_set_keep_above(GTK_WINDOW(window),TRUE);
    GtkWidget *box=gtk_box_new(GTK_ORIENTATION_VERTICAL,12);gtk_container_set_border_width(GTK_CONTAINER(box),18);gtk_container_add(GTK_CONTAINER(window),box);
    GtkWidget *label=gtk_label_new("Supervised access. Stop remains available without the service.");gtk_label_set_line_wrap(GTK_LABEL(label),TRUE);gtk_box_pack_start(GTK_BOX(box),label,FALSE,FALSE,0);
    GtkWidget *button=gtk_button_new_with_label("Stop computer control");gtk_box_pack_start(GTK_BOX(box),button,TRUE,TRUE,0);g_signal_connect(button,"clicked",G_CALLBACK(clicked),NULL);g_signal_connect(window,"delete-event",G_CALLBACK(closing),NULL);g_signal_connect(window,"key-press-event",G_CALLBACK(key_press),NULL);
    gtk_widget_show_all(window);message("ready",NULL,FALSE);g_thread_unref(g_thread_new("offgrid-consent-input",read_requests,NULL));gtk_main();g_dbus_connection_signal_unsubscribe(bus,subscription);g_bus_unwatch_name(watch);g_object_unref(bus);return 0;
}
