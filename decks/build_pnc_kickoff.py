#!/usr/bin/env python3
"""
PNC kickoff deck — TLD Redirect Engine.
Same visual style as the Dell Sentinel update decks (white bg, dark blue accent).
"""

from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.dml.color import RGBColor
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(SCRIPT_DIR, "pnc-2026-07-23-kickoff.pptx")

prs = Presentation()
prs.slide_width = Inches(13.333)
prs.slide_height = Inches(7.5)

BLACK = RGBColor(0x1A, 0x1A, 0x1A)
GRAY = RGBColor(0x55, 0x55, 0x55)
LIGHT_GRAY = RGBColor(0x88, 0x88, 0x88)
DARK_BLUE = RGBColor(0x00, 0x3D, 0x7A)
GREEN_OK = RGBColor(0x1E, 0x7E, 0x34)
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
LIGHT_GRAY_BG = RGBColor(0xF5, 0xF5, 0xF5)


def add_slide():
    slide = prs.slides.add_slide(prs.slide_layouts[6])
    bg = slide.background
    bg.fill.solid()
    bg.fill.fore_color.rgb = WHITE
    return slide


def add_title(slide, text, top=Inches(0.4), size=Pt(30), color=BLACK):
    box = slide.shapes.add_textbox(Inches(0.7), top, Inches(12), Inches(0.7))
    p = box.text_frame.paragraphs[0]
    p.text = text
    p.font.size = size
    p.font.color.rgb = color
    p.font.bold = True


def add_subtitle(slide, text, top=Inches(1.05), size=Pt(15), color=GRAY):
    box = slide.shapes.add_textbox(Inches(0.7), top, Inches(12), Inches(0.5))
    p = box.text_frame.paragraphs[0]
    p.text = text
    p.font.size = size
    p.font.color.rgb = color


def add_accent_bar(slide, top=Inches(1.0), width=Inches(3.0)):
    shape = slide.shapes.add_shape(1, Inches(0.7), top, width, Pt(2))
    shape.fill.solid()
    shape.fill.fore_color.rgb = DARK_BLUE
    shape.line.fill.background()


def add_body(slide, lines, top=Inches(1.7), left=Inches(0.7), width=Inches(11.9),
             size=Pt(15), line_spacing=Pt(10), color=BLACK):
    box = slide.shapes.add_textbox(left, top, width, Inches(5))
    tf = box.text_frame
    tf.word_wrap = True
    for i, line in enumerate(lines):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.text = line
        p.font.size = size
        p.font.color.rgb = color
        p.space_after = line_spacing


def add_table(slide, headers, rows, top, col_widths, row_height=Inches(0.42),
              body_size=Pt(12), header_size=Pt(13)):
    n_rows = len(rows) + 1
    n_cols = len(headers)
    shape = slide.shapes.add_table(n_rows, n_cols, Inches(0.7), top,
                                    sum(col_widths), row_height * n_rows)
    table = shape.table
    for i, w in enumerate(col_widths):
        table.columns[i].width = w
    for j, h in enumerate(headers):
        cell = table.cell(0, j)
        cell.text = h
        for p in cell.text_frame.paragraphs:
            p.font.size = header_size
            p.font.bold = True
            p.font.color.rgb = WHITE
        cell.fill.solid()
        cell.fill.fore_color.rgb = DARK_BLUE
    for i, row in enumerate(rows):
        for j, val in enumerate(row):
            cell = table.cell(i + 1, j)
            cell.text = str(val)
            for p in cell.text_frame.paragraphs:
                p.font.size = body_size
                p.font.color.rgb = BLACK
            if i % 2 == 1:
                cell.fill.solid()
                cell.fill.fore_color.rgb = LIGHT_GRAY_BG


def add_footer(slide):
    box = slide.shapes.add_textbox(Inches(0.7), Inches(7.1), Inches(12), Inches(0.3))
    p = box.text_frame.paragraphs[0]
    p.text = "Akamai TLD Redirect Engine  —  PNC Domain Consolidation  —  Kickoff 2026-07-23"
    p.font.size = Pt(10)
    p.font.color.rgb = LIGHT_GRAY


# ═══════════════════════════════════════════════════════════════════
# SLIDE 1 — Title
# ═══════════════════════════════════════════════════════════════════
slide = add_slide()
add_title(slide, "TLD Redirect Engine", top=Inches(2.4), size=Pt(44))
add_subtitle(slide, "PNC domain consolidation — project kickoff", top=Inches(3.25), size=Pt(24))
add_accent_bar(slide, top=Inches(3.1), width=Inches(3.5))
sub = slide.shapes.add_textbox(Inches(0.7), Inches(4.0), Inches(12), Inches(0.4))
p = sub.text_frame.paragraphs[0]
p.text = "2026-07-23  ·  today's actions  ·  what Akamai builds  ·  dependencies on PNC  ·  sequence"
p.font.size = Pt(16); p.font.color.rgb = GRAY
sub2 = slide.shapes.add_textbox(Inches(0.7), Inches(6.7), Inches(12), Inches(0.4))
p = sub2.text_frame.paragraphs[0]
p.text = "Brian Apley · Technical Solutions Architect · bapley@akamai.com"
p.font.size = Pt(12); p.font.color.rgb = LIGHT_GRAY

# ═══════════════════════════════════════════════════════════════════
# SLIDE 2 — Today's actions (two parallel tracks)
# ═══════════════════════════════════════════════════════════════════
slide = add_slide()
add_title(slide, "Today — two things start in parallel")
add_subtitle(slide, "Deal closed. We kick off the build now; PNC creates the one credential that unblocks admin access.")
add_accent_bar(slide)

add_body(slide, [
    "PNC — today:",
    "       •  Create the admin API token (a single strong secret). This is the only thing gating admin access to the redirect control plane.",
    "          Generate with:  openssl rand -hex 32  — share via your secrets channel, not email/chat.",
    "",
    "Akamai (Brian) — starting now:",
    "       •  Build the environment: control plane + data plane, database, cross-region rule sync, TLS provisioning, IaC (Terraform).",
    "       •  Everything on our side proceeds in parallel — the token is the only item that blocks first admin login, not the buildout.",
    "",
    "Goal for this call: agree the PNC-side dependencies below so nothing is discovered late.",
], top=Inches(1.55), size=Pt(15), line_spacing=Pt(7))
add_footer(slide)

# ═══════════════════════════════════════════════════════════════════
# SLIDE 3 — What Akamai builds
# ═══════════════════════════════════════════════════════════════════
slide = add_slide()
add_title(slide, "What Akamai builds")
add_subtitle(slide, "The engine that replaces the single-DC F5 redirect tier — proven in the pre-sales POC")
add_accent_bar(slide)

add_table(slide,
    ["Component", "What it does"],
    [
        ["Control plane",
         "Admin REST API + UI for domain/rule CRUD and bulk import/export. Provisions TLS certs. Publishes the authoritative rule set."],
        ["Data plane",
         "Serves the 301/302 redirects. Terminates TLS. Polls the shared rule store every ~5s — no restart to pick up rule changes."],
        ["Rule sync (Object Storage)",
         "Control plane writes rules.json on every change; all data-plane nodes converge within ~10s. No cross-region DB replication needed."],
        ["Automatic TLS",
         "Let's Encrypt via CertMagic, on-demand per domain — no 10-cert load-balancer limit. Scales to the full 2000+ domain inventory."],
        ["Observability",
         "Per-redirect beacon telemetry + analytics (top paths, referers, inactive-domain detection). Sink confirmed with PNC (see deps)."],
        ["Infrastructure as code",
         "All infra in Terraform, parameterized for PNC's environment. Reproducible stand-up and teardown; no console clicks."],
    ],
    top=Inches(1.6), col_widths=[Inches(3.2), Inches(8.7)],
    row_height=Inches(0.72), body_size=Pt(11.5))
add_footer(slide)

# ═══════════════════════════════════════════════════════════════════
# SLIDE 4 — Dependencies on PNC (the main ask)
# ═══════════════════════════════════════════════════════════════════
slide = add_slide()
add_title(slide, "Dependencies on PNC")
add_subtitle(slide, "What we need from your side — and roughly when we need it")
add_accent_bar(slide)

add_table(slide,
    ["#", "Dependency", "Owner", "When", "Notes"],
    [
        ["1", "Create the admin API token",
         "PNC", "Today",
         "openssl rand -hex 32. Only gate on admin access. Share via secrets channel."],
        ["2", "Provide the redirect inventory",
         "PNC", "Before go-live",
         "The domain -> target-URL -> status-code mapping (from F5). CSV/JSON; we bulk-import. This is the data that makes redirects real."],
        ["3", "Point DNS for testing",
         "PNC", "For validation",
         "CNAME/A a handful of test domains at the data plane so we can verify redirects + TLS end-to-end before the full cutover."],
        ["4", "Onboard the VMs to observability / SRE",
         "PNC", "Before go-live",
         "Add control + data-plane hosts to your monitoring, alerting, patching and on-call. Post-delivery these run in PNC's operational estate."],
        ["5", "Confirm the observability sink",
         "PNC", "Early",
         "Is Akamai DataStream 2 in your contract for the redirect beacon? If not, we point telemetry at a PNC-preferred sink (Splunk/syslog/etc.)."],
        ["6", "Network + firewall inputs",
         "PNC", "Early",
         "Admin-access source CIDRs (who reaches the control plane), and confirmation the data plane is reachable on 80/443 for ACME + serving."],
        ["7", "Data stores in PNC env",
         "PNC / Akamai", "Env buildout",
         "Confirm PostgreSQL + S3-compatible Object Storage availability in your compute env, or we provision alongside."],
    ],
    top=Inches(1.5), col_widths=[Inches(0.35), Inches(3.3), Inches(1.4), Inches(1.35), Inches(5.5)],
    row_height=Inches(0.62), body_size=Pt(10), header_size=Pt(11.5))
add_footer(slide)

# ═══════════════════════════════════════════════════════════════════
# SLIDE 5 — Sequence / what unblocks what
# ═══════════════════════════════════════════════════════════════════
slide = add_slide()
add_title(slide, "Sequence — what unblocks what")
add_subtitle(slide, "The critical path from today to F5 cutover")
add_accent_bar(slide)

add_body(slide, [
    "1.  Today  —  PNC creates the admin token; Akamai starts the environment buildout (parallel, no dependency between them).",
    "",
    "2.  Env buildout  —  Akamai stands up control + data plane, data stores, sync, TLS in PNC's compute environment.",
    "        Needs early:  observability sink decision (#5), network/firewall inputs (#6), data-store confirmation (#7).",
    "",
    "3.  Load + validate  —  PNC provides the redirect inventory (#2) and points test DNS (#3); Akamai bulk-imports and",
    "        verifies redirects + TLS end-to-end against the test domains.",
    "",
    "4.  Operational onboarding  —  PNC brings the VMs into observability / SRE / on-call (#4) so the system is supportable at go-live.",
    "",
    "5.  Cutover  —  dry-run the F5 migration, then move DNS in batches with rollback. Documented handoff to PNC operations.",
], top=Inches(1.55), size=Pt(14), line_spacing=Pt(6))

# closing ask line
close = slide.shapes.add_textbox(Inches(0.7), Inches(6.35), Inches(12), Inches(0.5))
p = close.text_frame.paragraphs[0]
p.text = "Today's decisions:  token owner + timing,  observability sink,  who owns DNS + SRE onboarding on PNC's side."
p.font.size = Pt(13); p.font.bold = True; p.font.color.rgb = DARK_BLUE
add_footer(slide)

prs.save(OUT)
print(f"Wrote: {OUT}")
print(f"Slides: {len(prs.slides)}")
