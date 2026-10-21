package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func usersCommand(_ context.Context, args []string, env Environment) error {
	presenter := Presenter{Stdout: env.Stdout, Stderr: env.Stderr, JSON: env.Options.JSON, Style: env.Style}
	cfg, err := localConfig(env)
	if err != nil {
		return err
	}

	// Check if multi-user mode is enabled
	if !cfg.MultiUserMode {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Users%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sSingle-user mode (default)%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sTo enable multi-user:%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %s$%s export OFFGRID_MULTI_USER=true\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sOr in config:%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    multi_user_mode: true\n")
		fmt.Fprintln(env.Stdout)
		return nil
	}

	if env.LocalAdmin.NewUserStore == nil {
		return fmt.Errorf("local user-store adapter unavailable")
	}
	store := env.LocalAdmin.NewUserStore(cfg.DataDir)

	if len(args) < 1 {
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s◈ Users%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "  %sMulti-user management%s\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUsage%s  offgrid users <command>\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sCommands%s\n", env.Style.Primary, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sList all users%s\n", "list", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sCreate a user%s\n", "create <name> <role>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sDelete a user%s\n", "delete <id>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sShow user info%s\n", "info <id>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sRegenerate API key%s\n", "reset-key <id>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintf(env.Stdout, "    %-24s %sShow quota usage%s\n", "quota <id>", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sRoles%s  admin, user, viewer, guest\n", env.Style.Dim, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		return nil
	}

	subCmd := args[0]
	switch subCmd {
	case "list":
		userList := store.ListUsers()
		if env.Options.JSON {
			publicUsers := make([]users.UserPublic, 0, len(userList))
			for _, u := range userList {
				publicUsers = append(publicUsers, u.ToPublic())
			}
			return localJSON(env, publicUsers)
		}
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sUsers%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		if len(userList) == 0 {
			fmt.Fprintln(env.Stdout, "  No users configured")
			fmt.Fprintln(env.Stdout)
			fmt.Fprintf(env.Stdout, "  %sCreate a user:%s offgrid users create <name> <role>\n", env.Style.Dim, env.Style.Reset)
		} else {
			fmt.Fprintf(env.Stdout, "  %s%-32s  %-10s  %-10s  %s%s\n", env.Style.Dim, "ID", "Username", "Role", "Created", env.Style.Reset)
			for _, u := range userList {
				status := ""
				if u.Disabled {
					status = " (disabled)"
				}
				fmt.Fprintf(env.Stdout, "  %-32s  %-10s  %-10s  %s%s\n", u.ID[:16]+"...", u.Username, u.Role, u.CreatedAt.Format("2006-01-02"), status)
			}
		}
		fmt.Fprintln(env.Stdout)

	case "create":
		if len(args) < 3 {
			presenter.Error("Usage: offgrid users create <username> <role>")
			return LegacyReportedError{}
		}
		username := args[1]
		role := users.Role(args[2])

		// Validate role
		validRoles := map[users.Role]bool{
			users.RoleAdmin: true, users.RoleUser: true,
			users.RoleViewer: true, users.RoleGuest: true,
		}
		if !validRoles[role] {
			presenter.Error("Invalid role. Use: admin, user, viewer, or guest")
			return LegacyReportedError{}
		}

		// Generate a strong one-time password for first login. Only its bcrypt
		// hash is persisted.
		if env.LocalAdmin.Entropy == nil {
			return fmt.Errorf("local password entropy unavailable")
		}
		passwordBytes := make([]byte, 18)
		if _, err := io.ReadFull(env.LocalAdmin.Entropy, passwordBytes); err != nil {
			presenter.Error(fmt.Sprintf("Failed to generate password: %v", err))
			return LegacyReportedError{}
		}
		password := base64.RawURLEncoding.EncodeToString(passwordBytes)

		user, apiKey, err := store.CreateUser(username, password, role)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to create user: %v", err))
			return LegacyReportedError{}
		}

		if env.Options.JSON {
			return localJSON(env, map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
				"api_key":  apiKey,
			})
		}

		fmt.Fprintln(env.Stdout)
		presenter.Success(fmt.Sprintf("Created user: %s", username))
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sUser ID:%s     %s\n", env.Style.Dim, env.Style.Reset, user.ID)
		fmt.Fprintf(env.Stdout, "  %sUsername:%s    %s\n", env.Style.Dim, env.Style.Reset, user.Username)
		fmt.Fprintf(env.Stdout, "  %sRole:%s        %s\n", env.Style.Dim, env.Style.Reset, user.Role)
		fmt.Fprintf(env.Stdout, "  %sPassword:%s    %s\n", env.Style.Dim, env.Style.Reset, password)
		fmt.Fprintf(env.Stdout, "  %sAPI Key:%s     %s\n", env.Style.Dim, env.Style.Reset, apiKey)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s⚠ Save the API key - it cannot be retrieved later%s\n", env.Style.Accent, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

	case "delete":
		if len(args) < 2 {
			presenter.Error("Usage: offgrid users delete <user-id>")
			return LegacyReportedError{}
		}
		userID := args[1]
		if err := store.DeleteUser(userID); err != nil {
			presenter.Error(fmt.Sprintf("Failed to delete user: %v", err))
			return LegacyReportedError{}
		}
		presenter.Success(fmt.Sprintf("Deleted user: %s", userID))

	case "info":
		if len(args) < 2 {
			presenter.Error("Usage: offgrid users info <user-id>")
			return LegacyReportedError{}
		}
		userID := args[1]
		user, ok := store.GetUser(userID)
		if !ok {
			presenter.Error("User not found")
			return LegacyReportedError{}
		}
		if env.Options.JSON {
			return localJSON(env, user.ToPublic())
		}
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sUser Info%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %sID:%s          %s\n", env.Style.Dim, env.Style.Reset, user.ID)
		fmt.Fprintf(env.Stdout, "  %sUsername:%s    %s\n", env.Style.Dim, env.Style.Reset, user.Username)
		fmt.Fprintf(env.Stdout, "  %sRole:%s        %s\n", env.Style.Dim, env.Style.Reset, user.Role)
		fmt.Fprintf(env.Stdout, "  %sCreated:%s     %s\n", env.Style.Dim, env.Style.Reset, user.CreatedAt.Format(time.RFC3339))
		if user.LastLoginAt != nil {
			fmt.Fprintf(env.Stdout, "  %sLast Login:%s  %s\n", env.Style.Dim, env.Style.Reset, user.LastLoginAt.Format(time.RFC3339))
		}
		fmt.Fprintf(env.Stdout, "  %sDisabled:%s    %v\n", env.Style.Dim, env.Style.Reset, user.Disabled)
		fmt.Fprintln(env.Stdout)

	case "reset-key":
		if len(args) < 2 {
			presenter.Error("Usage: offgrid users reset-key <user-id>")
			return LegacyReportedError{}
		}
		userID := args[1]
		apiKey, err := store.RegenerateAPIKey(userID)
		if err != nil {
			presenter.Error(fmt.Sprintf("Failed to regenerate API key: %v", err))
			return LegacyReportedError{}
		}
		if env.Options.JSON {
			return localJSON(env, map[string]string{"api_key": apiKey})
		}
		fmt.Fprintln(env.Stdout)
		presenter.Success("API key regenerated")
		fmt.Fprintf(env.Stdout, "  %sNew API Key:%s %s\n", env.Style.Dim, env.Style.Reset, apiKey)
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "  %s⚠ Save the API key - it cannot be retrieved later%s\n", env.Style.Accent, env.Style.Reset)
		fmt.Fprintln(env.Stdout)

	case "quota":
		if len(args) < 2 {
			presenter.Error("Usage: offgrid users quota <user-id>")
			return LegacyReportedError{}
		}
		userID := args[1]
		if env.LocalAdmin.NewQuotaManager == nil {
			return fmt.Errorf("local quota adapter unavailable")
		}
		quotaManager := env.LocalAdmin.NewQuotaManager(cfg.DataDir)
		summary := quotaManager.GetUsageSummary(userID)
		if env.Options.JSON {
			return localJSON(env, summary)
		}
		fmt.Fprintln(env.Stdout)
		fmt.Fprintf(env.Stdout, "%sQuota Usage%s\n", env.Style.Primary+env.Style.Bold, env.Style.Reset)
		fmt.Fprintln(env.Stdout)
		quotas, ok := summary["quotas"].([]map[string]any)
		if !ok || len(quotas) == 0 {
			fmt.Fprintln(env.Stdout, "  No quotas configured for this user")
		} else {
			for _, q := range quotas {
				exceeded := ""
				if ex, ok := q["exceeded"].(bool); ok && ex {
					exceeded = " " + env.Style.Error + "(EXCEEDED)" + env.Style.Reset
				}
				fmt.Fprintf(env.Stdout, "  %s (%s): %v / %v%s\n", q["type"], q["period"], q["current"], q["limit"], exceeded)
			}
		}
		fmt.Fprintln(env.Stdout)

	default:
		presenter.Error(fmt.Sprintf("Unknown users command: %s", subCmd))
		return LegacyReportedError{}
	}
	return nil
}

func RunUsersCommand(ctx context.Context, args []string, env Environment) error {
	return usersCommand(ctx, args, env)
}
