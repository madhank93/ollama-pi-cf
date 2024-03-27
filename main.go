package main

import (
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {

	pulumi.Run(func(ctx *pulumi.Context) error {

		// Generate a random password
		tunnelSecret, err := random.NewRandomPassword(ctx, "tunnelSecret", &random.RandomPasswordArgs{
			Length:  pulumi.Int(64),
			Special: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}

		// Create Cloudflare Tunnel
		tunnel, err := cloudflare.NewTunnel(ctx, "cloudflare-tunnel", &cloudflare.TunnelArgs{
			AccountId: pulumi.String("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"),
			Name:      pulumi.String("ai-tunnel"),
			Secret:    tunnelSecret.Result,
		})
		if err != nil {
			return err
		}

		ctx.Export("tunnelDomain", pulumi.Sprintf("%s.cfargotunnel.com", tunnel.ID()))

		// Create Cloudflare Record
		dnsRecord, err := cloudflare.NewRecord(ctx, "cloudflare-record", &cloudflare.RecordArgs{
			ZoneId:  pulumi.String("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"),
			Name:    pulumi.String("ai"),
			Value:   pulumi.Sprintf("%s.cfargotunnel.com", tunnel.ID()),
			Type:    pulumi.String("CNAME"),
			Proxied: pulumi.Bool(true),
		}, pulumi.DependsOn([]pulumi.Resource{tunnel}))
		if err != nil {
			return err
		}

		// Sub ansible variables
		ansibleSub, err := local.NewCommand(ctx, "substitute-cloudflare-env", &local.CommandArgs{
			Create: pulumi.String("cat cloudflare_vars.yml | envsubst > ansible/roles/cloudflare_tunnel/vars/cloudflare_vars.yml"),
			Environment: pulumi.StringMap{
				"TUNNEL_ID":     tunnel.ID(),
				"ACCOUNT_ID":    pulumi.String("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"),
				"TUNNEL_NAME":   tunnel.Name,
				"TUNNEL_SECRET": tunnelSecret.Result,
			},
		}, pulumi.DependsOn([]pulumi.Resource{dnsRecord}))
		if err != nil {
			return err
		}

		// Execute ansible commands
		ansibleRes, err := local.NewCommand(ctx, "run-ansible-play-book", &local.CommandArgs{
			Create: pulumi.String("cd ansible && ansible-playbook main.yml -i hosts"),
		}, pulumi.DependsOn([]pulumi.Resource{ansibleSub}))
		if err != nil {
			return err
		}

		ctx.Export("ansible-result-err", ansibleRes.Stderr)
		ctx.Export("ansible-result-out", ansibleRes.Stdout)

		return nil
	})
}
