// Disposable GTK document target. Private stdin/stdout only; no save, network,
// shell commands or interaction with another process. Not a production provider.
#include <gtk/gtk.h>
#include <json-glib/json-glib.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

typedef struct { guint id; gchar *text; gint start, end; } Document;
static Document documents[16];
static guint count=2, current=0, next_id=3, delay_ms=0, pending_creation=0;
static gint partial_budget=-1;
static gboolean loading=FALSE;
static GtkWidget *tabs, *editor, *focus_sink;
static GtkTextBuffer *buffer;

static void store_document(void) {
    GtkTextIter start,end,a,b;
    gtk_text_buffer_get_bounds(buffer,&start,&end);
    gchar *text=gtk_text_buffer_get_text(buffer,&start,&end,TRUE);
    g_free(documents[current].text); documents[current].text=text;
    if (!gtk_text_buffer_get_selection_bounds(buffer,&a,&b)) {
        gtk_text_buffer_get_iter_at_mark(buffer,&a,gtk_text_buffer_get_insert(buffer)); b=a;
    }
    documents[current].start=gtk_text_iter_get_offset(&a);
    documents[current].end=gtk_text_iter_get_offset(&b);
}
static void load_document(void) {
    loading=TRUE;
    gtk_text_buffer_set_text(buffer,documents[current].text,-1);
    GtkTextIter start,end;
    gtk_text_buffer_get_iter_at_offset(buffer,&start,documents[current].start);
    gtk_text_buffer_get_iter_at_offset(buffer,&end,documents[current].end);
    gtk_text_buffer_select_range(buffer,&start,&end);
    loading=FALSE;
}
static void switched(GtkNotebook *notebook,GtkWidget *page,guint index,gpointer data) {
    if (loading || index>=count) return;
    store_document(); current=index; load_document();
}
static void add_tab(void) {
    GtkWidget *page=gtk_box_new(GTK_ORIENTATION_VERTICAL,0);
    gtk_widget_show(page);
    gtk_notebook_append_page(GTK_NOTEBOOK(tabs),page,gtk_label_new("Untitled"));
}
static gboolean create_document(gpointer data) {
    pending_creation=0;
    if (count==16) return G_SOURCE_REMOVE;
    store_document(); loading=TRUE;
    documents[count]=(Document){next_id++,g_strdup(""),0,0}; add_tab(); count++;
    gtk_notebook_set_current_page(GTK_NOTEBOOK(tabs),(gint)count-1);
    current=count-1; load_document(); loading=FALSE;
    gtk_widget_grab_focus(editor);
    return G_SOURCE_REMOVE;
}
static void request_creation(GtkButton *button,gpointer data) {
    if (pending_creation) return;
    if (delay_ms) pending_creation=g_timeout_add(delay_ms,create_document,NULL);
    else create_document(NULL);
}
static void insert_fault(GtkTextBuffer *text_buffer,GtkTextIter *location,gchar *text,gint bytes,gpointer data) {
    if (loading || partial_budget<0) return;
    glong characters=g_utf8_strlen(text,bytes);
    if (characters<=partial_budget) { partial_budget-=(gint)characters; return; }
    gint allowed=partial_budget; partial_budget=0;
    g_signal_stop_emission_by_name(text_buffer,"insert-text");
    if (allowed>0) {
        const gchar *end=g_utf8_offset_to_pointer(text,allowed);
        loading=TRUE; gtk_text_buffer_insert(text_buffer,location,text,(gint)(end-text)); loading=FALSE;
    }
    gtk_widget_grab_focus(focus_sink);
}
static void respond(const gchar *kind) {
    store_document();
    JsonBuilder *b=json_builder_new(); json_builder_begin_object(b);
    #define STR(key,value) json_builder_set_member_name(b,key);json_builder_add_string_value(b,value)
    #define NUM(key,value) json_builder_set_member_name(b,key);json_builder_add_int_value(b,value)
    NUM("version",1);STR("kind",kind);NUM("pid",getpid());STR("editor","shared-editor");
    STR("selection_unit","scalar");NUM("selected",current);
    json_builder_set_member_name(b,"documents");json_builder_begin_array(b);
    for(guint i=0;i<count;i++) {
        json_builder_begin_object(b);NUM("id",documents[i].id);STR("text",documents[i].text);
        NUM("start",documents[i].start);NUM("end",documents[i].end);json_builder_end_object(b);
    }
    json_builder_end_array(b);json_builder_end_object(b);
    JsonNode *root=json_builder_get_root(b);JsonGenerator *generator=json_generator_new();
    json_generator_set_root(generator,root);gchar *line=json_generator_to_data(generator,NULL);
    puts(line);fflush(stdout);g_free(line);json_node_free(root);g_object_unref(generator);g_object_unref(b);
}
static gboolean integer(JsonObject *object,const gchar *name,gint max,gint *out) {
    if(!json_object_has_member(object,name))return FALSE;
    JsonNode *node=json_object_get_member(object,name);
    if(json_node_get_value_type(node)!=G_TYPE_INT64)return FALSE;
    gint64 value=json_node_get_int(node);if(value<0||value>max)return FALSE;*out=(gint)value;return TRUE;
}
static gboolean input(GIOChannel *channel,GIOCondition condition,gpointer data) {
    gchar *line=NULL;gsize length=0;
    if(g_io_channel_read_line(channel,&line,&length,NULL,NULL)!=G_IO_STATUS_NORMAL){gtk_main_quit();g_free(line);return G_SOURCE_REMOVE;}
    JsonParser *parser=json_parser_new();
    if(length>20000||!json_parser_load_from_data(parser,line,(gssize)length,NULL))goto done;
    JsonNode *root=json_parser_get_root(parser);if(!JSON_NODE_HOLDS_OBJECT(root))goto done;
    JsonObject *object=json_node_get_object(root);
    JsonNode *member=json_object_get_member(object,"kind");
    if(!member||json_node_get_value_type(member)!=G_TYPE_STRING)goto done;
    const gchar *kind=json_node_get_string(member);gint value,a,b;
    if(strcmp(kind,"state")==0){}
    else if(strcmp(kind,"new")==0)request_creation(NULL,NULL);
    else if(strcmp(kind,"select")==0&&integer(object,"index",(gint)count-1,&value))gtk_notebook_set_current_page(GTK_NOTEBOOK(tabs),value);
    else if(strcmp(kind,"delay")==0&&integer(object,"delay_ms",2000,&value))delay_ms=(guint)value;
    else if(strcmp(kind,"selection")==0&&integer(object,"start",gtk_text_buffer_get_char_count(buffer),&a)&&integer(object,"end",gtk_text_buffer_get_char_count(buffer),&b)&&a<=b){
        GtkTextIter start,end;gtk_text_buffer_get_iter_at_offset(buffer,&start,a);gtk_text_buffer_get_iter_at_offset(buffer,&end,b);gtk_text_buffer_select_range(buffer,&start,&end);
    }
    else if(strcmp(kind,"text")==0){
        JsonNode *node=json_object_get_member(object,"text");if(!node||json_node_get_value_type(node)!=G_TYPE_STRING)goto done;
        const gchar *text=json_node_get_string(node);if(strlen(text)>16000||!g_utf8_validate(text,-1,NULL))goto done;
        gtk_text_buffer_delete_selection(buffer,FALSE,TRUE);gtk_text_buffer_insert_at_cursor(buffer,text,-1);
    }
    else if(strcmp(kind,"partial_input")==0&&integer(object,"index",16000,&value))partial_budget=value;
    else if(strcmp(kind,"focus_loss")==0)gtk_widget_grab_focus(focus_sink);
    else if(strcmp(kind,"quit")==0)gtk_main_quit();
    else goto done;
    respond(kind);
done:
    g_object_unref(parser);g_free(line);return G_SOURCE_CONTINUE;
}
static gboolean ready(gpointer ignored){respond("ready");return G_SOURCE_REMOVE;}
static gboolean expire(gpointer ignored){gtk_main_quit();return G_SOURCE_REMOVE;}
int main(int argc,char **argv) {
    if(!gtk_init_check(&argc,&argv))return 2;
    GtkWidget *window=gtk_window_new(GTK_WINDOW_TOPLEVEL);gtk_window_set_title(GTK_WINDOW(window),"OffGrid disposable document fixture");gtk_window_set_default_size(GTK_WINDOW(window),680,380);
    GtkWidget *box=gtk_box_new(GTK_ORIENTATION_VERTICAL,12);gtk_container_set_border_width(GTK_CONTAINER(box),16);gtk_container_add(GTK_CONTAINER(window),box);
    documents[0]=(Document){1,g_strdup("Synthetic lifecycle content"),0,0};documents[1]=(Document){2,g_strdup("Untouched — café 日本語 12345"),0,0};
    tabs=gtk_notebook_new();add_tab();add_tab();gtk_box_pack_start(GTK_BOX(box),tabs,FALSE,FALSE,0);
    editor=gtk_text_view_new();buffer=gtk_text_view_get_buffer(GTK_TEXT_VIEW(editor));gtk_box_pack_start(GTK_BOX(box),editor,TRUE,TRUE,0);load_document();
    GtkWidget *button=gtk_button_new_with_label("New blank document");gtk_box_pack_start(GTK_BOX(box),button,FALSE,FALSE,0);
    focus_sink=gtk_entry_new();gtk_entry_set_text(GTK_ENTRY(focus_sink),"Owned focus-loss target");gtk_box_pack_start(GTK_BOX(box),focus_sink,FALSE,FALSE,0);
    g_signal_connect(tabs,"switch-page",G_CALLBACK(switched),NULL);g_signal_connect(button,"clicked",G_CALLBACK(request_creation),NULL);
    g_signal_connect(buffer,"insert-text",G_CALLBACK(insert_fault),NULL);g_signal_connect(window,"destroy",G_CALLBACK(gtk_main_quit),NULL);
    gtk_widget_show_all(window);gtk_window_present(GTK_WINDOW(window));gtk_widget_grab_focus(editor);
    GIOChannel *channel=g_io_channel_unix_new(0);g_io_add_watch(channel,G_IO_IN|G_IO_HUP,input,NULL);g_idle_add(ready,NULL);g_timeout_add_seconds(120,expire,NULL);gtk_main();
    g_io_channel_unref(channel);for(guint i=0;i<count;i++)g_free(documents[i].text);return 0;
}
