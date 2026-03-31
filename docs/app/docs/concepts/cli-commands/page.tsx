import Link from 'next/link'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { SiteHeader } from '@/components/site-header'
import { DocsSidebar } from '@/components/docs-sidebar'
import { CodeBlock } from '@/components/code-block'
import { getDocMetadata } from '@/config/docs-metadata'

export const metadata = getDocMetadata('/docs/concepts/cli-commands')

export default function CLICommandsPage() {
  return (
    <div className="min-h-screen bg-background">
      <SiteHeader />
      <DocsSidebar />

      <main className="lg:pl-64">
        <div className="max-w-4xl mx-auto py-12 px-6 lg:px-8">
          <div className="mb-14">
            <p className="text-sm font-mono font-medium text-primary mb-3 tracking-wide uppercase">
              Core Concepts
            </p>
            <h1 className="text-3xl lg:text-4xl font-bold tracking-tight text-foreground mb-4">
              New CLI Commands
            </h1>
            <p className="text-lg text-muted-foreground leading-relaxed max-w-2xl">
              Jua v3 adds operational commands inspired by Laravel/Goravel:
              route listing, maintenance mode, and one-command deployment.
            </p>
          </div>

          {/* jua routes */}
          <div className="mb-14">
            <h2 className="text-xl font-semibold text-foreground mb-4">
              <code className="text-primary font-mono">jua routes</code>
            </h2>
            <p className="text-muted-foreground mb-6">
              List all registered API routes in a formatted table. Parses your{' '}
              <code className="text-primary bg-accent/30 px-1.5 py-0.5 rounded text-[13px]">routes.go</code> file
              and shows the HTTP method, path, handler function, and middleware group.
            </p>
            <CodeBlock language="bash" filename="Terminal" code={`$ jua routes

  METHOD  PATH                              HANDLER                GROUP
  ──────  ────                              ───────                ─────
  GET     /api/health                       func1                  public
  POST    /api/auth/register                authHandler.Register   public
  POST    /api/auth/login                   authHandler.Login      public
  POST    /api/auth/refresh                 authHandler.Refresh    public
  GET     /api/auth/me                      authHandler.Me         protected
  POST    /api/auth/logout                  authHandler.Logout     protected
  POST    /api/auth/totp/setup              totpHandler.Setup      protected
  GET     /api/auth/totp/status             totpHandler.Status     protected
  GET     /api/users/:id                    userHandler.GetByID    protected
  POST    /api/uploads                      uploadHandler.Create   protected
  POST    /api/ai/chat                      aiHandler.Chat         protected
  DELETE  /api/admin/users/:id              userHandler.Delete     admin

  16 routes total`} />
            <p className="text-sm text-muted-foreground/70 mt-4">
              Works for both monorepo (<code className="text-muted-foreground">apps/api/internal/routes/</code>) and
              single app (<code className="text-muted-foreground">internal/routes/</code>) projects.
            </p>
          </div>

          {/* jua down / jua up */}
          <div className="mb-14">
            <h2 className="text-xl font-semibold text-foreground mb-4">
              <code className="text-primary font-mono">jua down</code> / <code className="text-primary font-mono">jua up</code>
            </h2>
            <p className="text-muted-foreground mb-6">
              Toggle maintenance mode. When enabled, all API requests receive a{' '}
              <code className="text-primary bg-accent/30 px-1.5 py-0.5 rounded text-[13px]">503 Service Unavailable</code> response.
            </p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
              <CodeBlock language="bash" filename="Enable maintenance" code={`$ jua down

  Application is now in maintenance mode.
  All requests will receive 503.
  Run 'jua up' to bring it back online.`} className="" />
              <CodeBlock language="bash" filename="Disable maintenance" code={`$ jua up

  Application is back online!
  Normal request handling has resumed.`} className="" />
            </div>

            <div className="rounded-lg border border-border/40 bg-accent/20 p-5">
              <h3 className="text-sm font-semibold text-foreground mb-2">How it works</h3>
              <ul className="space-y-2 text-sm text-muted-foreground">
                <li className="flex items-start gap-2">
                  <span className="text-primary mt-0.5">1.</span>
                  <code className="text-foreground/80">jua down</code> creates a <code className="text-foreground/80">.maintenance</code> file in the project root
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-primary mt-0.5">2.</span>
                  The scaffolded <code className="text-foreground/80">Maintenance()</code> middleware checks for this file on every request
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-primary mt-0.5">3.</span>
                  <code className="text-foreground/80">jua up</code> removes the file, resuming normal operation
                </li>
              </ul>
            </div>

            <CodeBlock language="go" filename="middleware/maintenance.go" className="mt-6" code={`func Maintenance() gin.HandlerFunc {
    return func(c *gin.Context) {
        if _, err := os.Stat(".maintenance"); err == nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "error": gin.H{
                    "code":    "MAINTENANCE",
                    "message": "Application is in maintenance mode.",
                },
            })
            c.Abort()
            return
        }
        c.Next()
    }
}`} highlightLines={[3, 4, 5, 6, 7, 8, 9]} />
          </div>

          {/* jua deploy */}
          <div className="mb-14">
            <h2 className="text-xl font-semibold text-foreground mb-4">
              <code className="text-primary font-mono">jua deploy</code>
            </h2>
            <p className="text-muted-foreground mb-4">
              One-command production deployment. See the dedicated{' '}
              <Link href="/docs/infrastructure/deploy-command" className="text-primary hover:underline">
                Deploy Command
              </Link>{' '}
              guide for full details.
            </p>
            <CodeBlock language="bash" filename="Terminal" code={`# Deploy with flags
jua deploy --host user@server.com --domain myapp.com

# Or set env vars in .env
DEPLOY_HOST=user@server.com
DEPLOY_DOMAIN=myapp.com
DEPLOY_KEY_FILE=~/.ssh/id_rsa

jua deploy`} />
          </div>

          {/* Complete command reference */}
          <div className="mb-14">
            <h2 className="text-xl font-semibold text-foreground mb-4">Complete command reference</h2>
            <div className="overflow-x-auto rounded-lg border border-border/40">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border/40 bg-accent/20">
                    <th className="px-4 py-3 text-left font-medium text-foreground/80">Command</th>
                    <th className="px-4 py-3 text-left font-medium text-foreground/80">Description</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/30">
                  {[
                    ['jua new <name>', 'Create a new project (interactive)'],
                    ['jua new-desktop <name>', 'Create a Wails desktop app'],
                    ['jua generate resource <Name>', 'Generate full-stack CRUD resource'],
                    ['jua remove resource <Name>', 'Remove a generated resource'],
                    ['jua add role <ROLE>', 'Add a new role to the project'],
                    ['jua sync', 'Sync Go types to TypeScript'],
                    ['jua start', 'Start development servers'],
                    ['jua start server', 'Start Go API only'],
                    ['jua start client', 'Start frontend only'],
                    ['jua compile', 'Build desktop app (Wails)'],
                    ['jua studio', 'Open GORM Studio'],
                    ['jua migrate', 'Run database migrations'],
                    ['jua migrate --fresh', 'Drop all tables + re-migrate'],
                    ['jua seed', 'Run database seeders'],
                    ['jua routes', 'List all API routes'],
                    ['jua down', 'Enable maintenance mode (503)'],
                    ['jua up', 'Disable maintenance mode'],
                    ['jua deploy', 'Deploy to production server'],
                    ['jua upgrade', 'Upgrade project templates'],
                    ['jua update', 'Update Jua CLI to latest'],
                    ['jua version', 'Print CLI version'],
                  ].map(([cmd, desc]) => (
                    <tr key={cmd} className="hover:bg-accent/20 transition-colors">
                      <td className="px-4 py-2.5 font-mono text-[13px] text-primary whitespace-nowrap">{cmd}</td>
                      <td className="px-4 py-2.5 text-muted-foreground">{desc}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div className="flex items-center justify-between pt-8 border-t border-border/40">
            <Button variant="ghost" size="sm" asChild className="text-muted-foreground/70 hover:text-foreground">
              <Link href="/docs/concepts/cli" className="gap-1.5">
                <ArrowLeft className="h-3.5 w-3.5" />
                CLI Commands
              </Link>
            </Button>
            <Button variant="ghost" size="sm" asChild className="text-muted-foreground/70 hover:text-foreground">
              <Link href="/docs/concepts/code-generation" className="gap-1.5">
                Code Generation
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </div>
        </div>
      </main>
    </div>
  )
}
