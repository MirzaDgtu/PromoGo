package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MirzaDgtu/PromoGo/internal/config"
	"github.com/MirzaDgtu/PromoGo/internal/migrate"
	"github.com/MirzaDgtu/PromoGo/internal/repository/postgres"
	"github.com/MirzaDgtu/PromoGo/internal/service"
)

// runBootstrapAdmin creates (or promotes) the first platform_admin
// StaffMembership directly against Postgres, bypassing the HTTP admin API —
// see knowledge/Decisions.md DEC-009 for why this exists: every admin API
// endpoint that can grant a StaffMembership itself requires a caller who
// already holds staff.manage.
func runBootstrapAdmin(args []string) {
	fs := flag.NewFlagSet("bootstrap-admin", flag.ExitOnError)
	subject := fs.String("subject", "", "OIDC subject (sub claim) of the platform admin to bootstrap (required)")
	email := fs.String("email", "", "email address recorded for the staff user (required)")
	displayName := fs.String("name", "", "display name recorded for the staff user (required)")
	orgName := fs.String("org-name", "Default Organization", "organization name to grant platform_admin in (ignored if --org-id is set)")
	orgID := fs.Int64("org-id", 0, "organization ID to grant platform_admin in (takes precedence over --org-name)")
	fs.Parse(args)

	if *subject == "" || *email == "" || *displayName == "" {
		fmt.Fprintln(os.Stderr, "bootstrap-admin: --subject, --email, and --name are required")
		fs.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN(), cfg.Postgres.MaxConns)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	// bootstrap-admin may be the very first thing run against a freshly
	// provisioned database; migrate.Run is idempotent and
	// advisory-lock-guarded, so applying it here is safe even if the
	// server has already migrated.
	if err := migrate.Run(ctx, cfg.Postgres.DSN()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	orgs := postgres.NewOrganizationRepository(pool)
	users := postgres.NewStaffUserRepository(pool)
	memberships := postgres.NewStaffMembershipRepository(pool)

	result, err := service.BootstrapPlatformAdmin(ctx, orgs, users, memberships, service.BootstrapAdminInput{
		Subject:     *subject,
		Email:       *email,
		DisplayName: *displayName,
		OrgName:     *orgName,
		OrgID:       *orgID,
	})
	if err != nil {
		log.Fatalf("bootstrap-admin: %v", err)
	}

	fmt.Printf("organization: id=%d name=%q\n", result.Organization.ID, result.Organization.Name)
	fmt.Printf("staff_user:   id=%d external_subject=%q created=%v\n", result.StaffUser.ID, result.StaffUser.ExternalSubject, result.UserCreated)
	fmt.Printf("membership:   id=%d role=%s created=%v\n", result.Membership.ID, result.Membership.Role, result.MembershipCreated)
}
