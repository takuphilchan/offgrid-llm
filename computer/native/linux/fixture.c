#include <gtk/gtk.h>
#include <stdio.h>
#include <string.h>
static GtkWidget *entry;
static gboolean input(GIOChannel *channel,GIOCondition condition,gpointer data){
    gchar *line=NULL;gsize length=0;if(g_io_channel_read_line(channel,&line,&length,NULL,NULL)!=G_IO_STATUS_NORMAL){gtk_main_quit();return FALSE;}
    if(strcmp(line,"read\n")==0){printf("VALUE %s\n",gtk_entry_get_text(GTK_ENTRY(entry)));fflush(stdout);}
    else if(strcmp(line,"manual\n")==0){gtk_entry_set_text(GTK_ENTRY(entry),"Manual fixture edit");puts("MANUAL");fflush(stdout);}
    g_free(line);return TRUE;
}
static gboolean ready(gpointer ignored){puts("READY");fflush(stdout);return G_SOURCE_REMOVE;}
int main(int argc,char **argv){
    if(!gtk_init_check(&argc,&argv))return 1;
    GtkWidget *window=gtk_window_new(GTK_WINDOW_TOPLEVEL);gtk_window_set_title(GTK_WINDOW(window),"OffGrid AT-SPI fixture");gtk_window_set_default_size(GTK_WINDOW(window),560,220);
    GtkWidget *box=gtk_box_new(GTK_ORIENTATION_VERTICAL,12);gtk_container_set_border_width(GTK_CONTAINER(box),20);gtk_container_add(GTK_CONTAINER(window),box);
    entry=gtk_entry_new();gtk_entry_set_text(GTK_ENTRY(entry),"Original fixture text");atk_object_set_name(gtk_widget_get_accessible(entry),"Report title");gtk_box_pack_start(GTK_BOX(box),entry,FALSE,FALSE,0);
    GtkWidget *password=gtk_entry_new();gtk_entry_set_visibility(GTK_ENTRY(password),FALSE);gtk_entry_set_text(GTK_ENTRY(password),"SYNTHETIC_TEST_SECRET");gtk_box_pack_start(GTK_BOX(box),password,FALSE,FALSE,0);
    g_signal_connect(window,"destroy",G_CALLBACK(gtk_main_quit),NULL);gtk_widget_show_all(window);gtk_window_present(GTK_WINDOW(window));gtk_widget_grab_focus(entry);
    GIOChannel *channel=g_io_channel_unix_new(0);g_io_add_watch(channel,G_IO_IN|G_IO_HUP,input,NULL);g_idle_add(ready,NULL);gtk_main();g_io_channel_unref(channel);return 0;
}
