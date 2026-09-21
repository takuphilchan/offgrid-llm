//go:build linux && cgo && offgrid_native

package computer

/*
#cgo pkg-config: atspi-2 gobject-2.0
#include <atspi/atspi.h>
#include <stdlib.h>
static void og_unref(void *value){if(value)g_object_unref(value);}
static void og_ref(void *value){if(value)g_object_ref(value);}
static int og_state(AtspiAccessible *element, AtspiStateType state){
    AtspiStateSet *states=atspi_accessible_get_state_set(element);
    if(!states)return 0;int present=atspi_state_set_contains(states,state);g_object_unref(states);return present;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type linuxSurface struct {
	target     NativeTarget
	object     *C.AtspiAccessible
	pid        int
	generation string
}
type linuxControl struct {
	object                *C.AtspiAccessible
	info                  NativeElement
	identity, fingerprint string
	action                int
}
type NativeLinux struct {
	targets  map[string]*linuxSurface
	selected *linuxSurface
	controls map[string]linuxControl
	prepared map[string]PreparedControlAction
	observed ControlObservation
	sequence uint64
}

var _ NativeAdapter = (*NativeLinux)(nil)

func NewNativeLinux() (*NativeLinux, error) {
	if os.Getuid() == 0 || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" || (os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "") {
		return nil, fmt.Errorf("computer_desktop_unavailable")
	}
	if C.atspi_init() != 0 {
		return nil, fmt.Errorf("computer_permission_denied")
	}
	C.atspi_set_timeout(2000, 2000)
	return &NativeLinux{targets: map[string]*linuxSurface{}, controls: map[string]linuxControl{}, prepared: map[string]PreparedControlAction{}}, nil
}
func atspiError(err *C.GError) bool {
	if err == nil {
		return false
	}
	C.g_error_free(err)
	return true
}
func atspiString(value *C.char) string {
	if value == nil {
		return ""
	}
	defer C.g_free(C.gpointer(unsafe.Pointer(value)))
	return C.GoString(value)
}
func linuxGeneration(pid int) (string, error) {
	if pid <= 0 || pid == os.Getpid() {
		return "", ErrControlScope
	}
	status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "", ErrControlScope
	}
	owned := false
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[0] == "Uid:" {
			uid := strconv.Itoa(os.Getuid())
			owned = fields[1] == uid && fields[2] == uid && fields[3] == uid && fields[4] == uid
		}
		if len(fields) == 2 && fields[0] == "CapEff:" {
			bits, e := strconv.ParseUint(fields[1], 16, 64)
			if e != nil || bits != 0 {
				return "", ErrControlScope
			}
		}
	}
	if !owned {
		return "", ErrControlScope
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", ErrControlScope
	}
	end := strings.LastIndexByte(string(stat), ')')
	if end < 0 {
		return "", ErrControlScope
	}
	fields := strings.Fields(string(stat)[end+1:])
	if len(fields) < 20 {
		return "", ErrControlScope
	}
	return fmt.Sprintf("%d:%s", pid, fields[19]), nil
}
func (d *NativeLinux) clear() {
	for _, control := range d.controls {
		C.og_unref(unsafe.Pointer(control.object))
	}
	d.controls = map[string]linuxControl{}
	d.prepared = map[string]PreparedControlAction{}
	d.observed = ControlObservation{}
}
func (d *NativeLinux) Close() {
	d.clear()
	for _, target := range d.targets {
		C.og_unref(unsafe.Pointer(target.object))
	}
	d.targets = nil
	d.selected = nil
	C.atspi_exit()
}
func (d *NativeLinux) Targets() ([]NativeTarget, error) {
	if d.selected != nil {
		return nil, ErrControlScope
	}
	for _, target := range d.targets {
		C.og_unref(unsafe.Pointer(target.object))
	}
	d.targets = map[string]*linuxSurface{}
	desktop := C.atspi_get_desktop(0)
	if desktop == nil {
		return nil, fmt.Errorf("computer_desktop_unavailable")
	}
	defer C.og_unref(unsafe.Pointer(desktop))
	C.atspi_accessible_clear_cache(desktop)
	var e *C.GError
	count := C.atspi_accessible_get_child_count(desktop, &e)
	if atspiError(e) {
		return nil, ErrControlScope
	}
	result := []NativeTarget{}
	for index := C.int(0); index < count && index < 128 && len(result) < 100; index++ {
		e = nil
		app := C.atspi_accessible_get_child_at_index(desktop, index, &e)
		if atspiError(e) || app == nil {
			continue
		}
		e = nil
		pid := int(C.atspi_accessible_get_process_id(app, &e))
		if atspiError(e) {
			C.og_unref(unsafe.Pointer(app))
			continue
		}
		generation, err := linuxGeneration(pid)
		if err != nil {
			C.og_unref(unsafe.Pointer(app))
			continue
		}
		e = nil
		windows := C.atspi_accessible_get_child_count(app, &e)
		if atspiError(e) {
			C.og_unref(unsafe.Pointer(app))
			continue
		}
		for n := C.int(0); n < windows && n < 100 && len(result) < 100; n++ {
			e = nil
			window := C.atspi_accessible_get_child_at_index(app, n, &e)
			if atspiError(e) || window == nil {
				continue
			}
			e = nil
			role := C.atspi_accessible_get_role(window, &e)
			if atspiError(e) || (role != C.ATSPI_ROLE_FRAME && role != C.ATSPI_ROLE_DIALOG) || C.og_state(window, C.ATSPI_STATE_SHOWING) == 0 {
				C.og_unref(unsafe.Pointer(window))
				continue
			}
			e = nil
			title := atspiString(C.atspi_accessible_get_name(window, &e))
			if atspiError(e) || title == "" || len(title) > 1024 {
				C.og_unref(unsafe.Pointer(window))
				continue
			}
			target := NativeTarget{Identity: TargetIdentity{ID: nativeID(), OSSession: fmt.Sprintf("linux:%d:desktop", os.Getuid()), ProcessGeneration: generation, Surface: nativeID(), Driver: "linux-atspi"}, Title: title}
			d.targets[target.Identity.ID] = &linuxSurface{target: target, object: window, pid: pid, generation: generation}
			result = append(result, target)
		}
		C.og_unref(unsafe.Pointer(app))
	}
	return result, nil
}
func (d *NativeLinux) check(focus bool) error {
	if d.selected == nil {
		return ErrControlScope
	}
	generation, err := linuxGeneration(d.selected.pid)
	if err != nil || generation != d.selected.generation {
		return ErrControlScope
	}
	C.atspi_accessible_clear_cache(d.selected.object)
	if C.og_state(d.selected.object, C.ATSPI_STATE_DEFUNCT) != 0 || C.og_state(d.selected.object, C.ATSPI_STATE_SHOWING) == 0 {
		return ErrControlScope
	}
	if focus && C.og_state(d.selected.object, C.ATSPI_STATE_ACTIVE) == 0 {
		return ErrControlScope
	}
	return nil
}
func (d *NativeLinux) Select(id string) (NativeTarget, error) {
	if d.selected != nil {
		return NativeTarget{}, ErrControlScope
	}
	target := d.targets[id]
	if target == nil {
		return NativeTarget{}, ErrControlScope
	}
	d.selected = target
	if err := d.check(false); err != nil {
		d.selected = nil
		return NativeTarget{}, err
	}
	return target.target, nil
}
func (d *NativeLinux) within(object *C.AtspiAccessible) bool {
	C.og_ref(unsafe.Pointer(object))
	current := object
	for depth := 0; depth < 128; depth++ {
		if current == d.selected.object {
			C.og_unref(unsafe.Pointer(current))
			return true
		}
		var e *C.GError
		parent := C.atspi_accessible_get_parent(current, &e)
		C.og_unref(unsafe.Pointer(current))
		if atspiError(e) || parent == nil {
			return false
		}
		current = parent
	}
	C.og_unref(unsafe.Pointer(current))
	return false
}
func (d *NativeLinux) inspect(object *C.AtspiAccessible) (linuxControl, error) {
	C.atspi_accessible_clear_cache(object)
	var e *C.GError
	pid := int(C.atspi_accessible_get_process_id(object, &e))
	if atspiError(e) || pid != d.selected.pid {
		return linuxControl{}, ErrControlScope
	}
	e = nil
	role := C.atspi_accessible_get_role(object, &e)
	if atspiError(e) || role == C.ATSPI_ROLE_PASSWORD_TEXT || C.og_state(object, C.ATSPI_STATE_DEFUNCT) != 0 || C.og_state(object, C.ATSPI_STATE_SENSITIVE) == 0 || C.og_state(object, C.ATSPI_STATE_ENABLED) == 0 || C.og_state(object, C.ATSPI_STATE_SHOWING) == 0 {
		return linuxControl{}, ErrControlScope
	}
	e = nil
	name := atspiString(C.atspi_accessible_get_name(object, &e))
	if atspiError(e) || len(name) > 1024 {
		return linuxControl{}, ErrControlScope
	}
	for _, word := range []string{"password", "passcode", "credential", "credit card", "verification code", "security code", "token"} {
		if strings.Contains(strings.ToLower(name), word) {
			return linuxControl{}, ErrControlScope
		}
	}
	e = nil
	roleName := atspiString(C.atspi_accessible_get_role_name(object, &e))
	if atspiError(e) {
		return linuxControl{}, ErrControlScope
	}
	info := NativeElement{Name: name, Role: roleName}
	value := ""
	editable := C.atspi_accessible_get_editable_text_iface(object)
	if editable != nil {
		info.Writable = true
		C.og_unref(unsafe.Pointer(editable))
	}
	if text := C.atspi_accessible_get_text_iface(object); text != nil {
		e = nil
		length := C.atspi_text_get_character_count(text, &e)
		if atspiError(e) || length > 1048576 {
			C.og_unref(unsafe.Pointer(text))
			return linuxControl{}, ErrControlScope
		}
		e = nil
		value = atspiString(C.atspi_text_get_text(text, 0, length, &e))
		C.og_unref(unsafe.Pointer(text))
		if atspiError(e) {
			return linuxControl{}, ErrControlScope
		}
	}
	actionIndex := -1
	if action := C.atspi_accessible_get_action_iface(object); action != nil {
		e = nil
		count := C.atspi_action_get_n_actions(action, &e)
		if !atspiError(e) {
			for index := C.int(0); index < count && index < 16; index++ {
				e = nil
				name := atspiString(C.atspi_action_get_action_name(action, index, &e))
				if atspiError(e) {
					break
				}
				if name == "click" || name == "press" || name == "activate" {
					actionIndex = int(index)
					break
				}
			}
		}
		C.og_unref(unsafe.Pointer(action))
		info.Invokable = actionIndex >= 0
	}
	return linuxControl{object: object, info: info, action: actionIndex, fingerprint: nativeHash([]any{name, roleName, info.Writable, info.Invokable, value, actionIndex})}, nil
}
func (d *NativeLinux) Observe(ctx context.Context) (NativeView, error) {
	if err := d.check(false); err != nil {
		return NativeView{}, err
	}
	d.clear()
	C.og_ref(unsafe.Pointer(d.selected.object))
	queue := []*C.AtspiAccessible{d.selected.object}
	defer func() {
		for _, object := range queue {
			C.og_unref(unsafe.Pointer(object))
		}
	}()
	view := NativeView{Elements: []NativeElement{}}
	digests := []string{}
	visited := 0
	for len(queue) > 0 && visited < 400 {
		if err := ctx.Err(); err != nil {
			return NativeView{}, err
		}
		object := queue[0]
		queue = queue[1:]
		visited++
		if control, err := d.inspect(object); err == nil {
			control.info.ID = nativeID()
			control.identity = nativeID()
			C.og_ref(unsafe.Pointer(object))
			d.controls[control.info.ID] = control
			view.Elements = append(view.Elements, control.info)
			digests = append(digests, control.fingerprint)
		}
		var e *C.GError
		count := C.atspi_accessible_get_child_count(object, &e)
		if !atspiError(e) {
			for n := C.int(0); n < count; n++ {
				if len(queue)+visited >= 400 {
					view.Limited = true
					break
				}
				e = nil
				child := C.atspi_accessible_get_child_at_index(object, n, &e)
				if !atspiError(e) && child != nil {
					queue = append(queue, child)
				}
			}
		}
		C.og_unref(unsafe.Pointer(object))
	}
	d.sequence++
	view.Observation = ControlObservation{ID: nativeID(), Target: d.selected.target.Identity, Sequence: d.sequence, CapturedAt: time.Now().UTC(), StateDigest: nativeHash(digests)}
	d.observed = view.Observation
	return view, nil
}
func (d *NativeLinux) Prepare(binding ControlBinding, op Operation, observation ControlObservation) (PreparedControlAction, error) {
	if err := d.check(false); err != nil {
		return PreparedControlAction{}, err
	}
	if binding.Validate() != nil || binding.Target != d.selected.target.Identity || op.Validate() != nil {
		return PreparedControlAction{}, ErrControlScope
	}
	if observation != d.observed || observation.Validate(binding.Target, time.Now()) != nil {
		return PreparedControlAction{}, ErrStaleObservation
	}
	control, ok := d.controls[op.Element]
	if !ok || !d.within(control.object) {
		return PreparedControlAction{}, ErrControlScope
	}
	fresh, err := d.inspect(control.object)
	if err != nil || fresh.fingerprint != control.fingerprint {
		return PreparedControlAction{}, ErrStaleObservation
	}
	if op.Kind != "replace_text" && op.Kind != "activate" || op.Kind == "replace_text" && !fresh.info.Writable || op.Kind == "activate" && !fresh.info.Invokable {
		return PreparedControlAction{}, ErrInvalidControl
	}
	action := PreparedControlAction{ID: nativeID(), Operation: op, ControlIdentity: control.identity, Precondition: control.fingerprint, ExpectedChange: nativeHash(op)}
	d.prepared[action.ID] = action
	return action, nil
}
func (d *NativeLinux) ApprovalSummary(step BoundedStep) (string, error) {
	if err := d.check(false); err != nil {
		return "", err
	}
	if _, err := step.Digest(); err != nil {
		return "", err
	}
	if step.Binding.Target != d.selected.target.Identity {
		return "", ErrControlScope
	}
	text := "Approve these exact changes in " + strconv.Quote(d.selected.target.Title) + "?\nOffGrid will focus this application.\n\n"
	for _, action := range step.Actions {
		saved, ok := d.prepared[action.ID]
		if !ok || nativeHash(saved) != nativeHash(action) {
			return "", ErrControlApproval
		}
		control := d.controls[action.Operation.Element]
		if action.Operation.Kind == "replace_text" {
			text += "Replace all text in " + strconv.Quote(control.info.Name) + " with:\n" + strconv.Quote(*action.Operation.Text) + "\n\n"
		} else {
			text += "Activate " + strconv.Quote(control.info.Name) + ". This may submit or change information; dispatch alone does not verify the outcome.\n\n"
		}
	}
	return text + "Only these changes are allowed. Use the local Stop control to revoke access.", nil
}
func (d *NativeLinux) FocusSelected() error {
	if err := d.check(false); err != nil {
		return err
	}
	if d.check(true) == nil {
		return nil
	}
	component := C.atspi_accessible_get_component_iface(d.selected.object)
	if component == nil {
		return ErrControlScope
	}
	defer C.og_unref(unsafe.Pointer(component))
	var e *C.GError
	ok := C.atspi_component_grab_focus(component, &e)
	if atspiError(e) {
		return ErrControlScope
	}
	_ = ok
	for attempt := 0; attempt < 5; attempt++ {
		if d.check(true) == nil {
			return nil
		}
		time.Sleep(40 * time.Millisecond)
	}
	return ErrControlScope
}
func (d *NativeLinux) Dispatch(ctx context.Context, action PreparedControlAction) (ControlResult, error) {
	result := ControlResult{ActionID: action.ID, Outcome: "failed"}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := d.check(true); err != nil {
		return result, err
	}
	if saved, ok := d.prepared[action.ID]; !ok || nativeHash(saved) != nativeHash(action) {
		return result, ErrControlApproval
	}
	delete(d.prepared, action.ID)
	control, ok := d.controls[action.Operation.Element]
	if !ok || !d.within(control.object) {
		return result, ErrControlScope
	}
	fresh, err := d.inspect(control.object)
	if err != nil || fresh.fingerprint != action.Precondition || control.identity != action.ControlIdentity {
		return result, ErrStaleObservation
	}
	d.observed = ControlObservation{}
	var e *C.GError
	if action.Operation.Kind == "replace_text" {
		editable := C.atspi_accessible_get_editable_text_iface(control.object)
		if editable == nil {
			return result, ErrControlScope
		}
		defer C.og_unref(unsafe.Pointer(editable))
		value := C.CString(*action.Operation.Text)
		defer C.free(unsafe.Pointer(value))
		result.Outcome = "uncertain"
		result.Uncertain = true
		ok := C.atspi_editable_text_set_text_contents(editable, value, &e)
		if atspiError(e) || ok == 0 {
			return result, ErrUncertain
		}
		text := C.atspi_accessible_get_text_iface(control.object)
		if text == nil {
			return result, ErrUncertain
		}
		defer C.og_unref(unsafe.Pointer(text))
		e = nil
		actual := atspiString(C.atspi_text_get_text(text, 0, -1, &e))
		if atspiError(e) || actual != *action.Operation.Text {
			return result, ErrUncertain
		}
		result.Outcome = "verified"
		result.Uncertain = false
		result.EvidenceID = nativeHash(actual)
		return result, nil
	}
	call := C.atspi_accessible_get_action_iface(control.object)
	if call == nil {
		return result, ErrControlScope
	}
	defer C.og_unref(unsafe.Pointer(call))
	result.Outcome = "uncertain"
	result.Uncertain = true
	okay := C.atspi_action_do_action(call, C.int(control.action), &e)
	if atspiError(e) || okay == 0 {
		return result, ErrUncertain
	}
	result.Outcome = "dispatched"
	result.Uncertain = false
	return result, nil
}
