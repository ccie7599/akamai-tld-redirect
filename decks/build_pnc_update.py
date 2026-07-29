#!/usr/bin/env python3
"""PNC delivery-update deck (follow-up to the 2026-07-23 kickoff)."""
from pptx import Presentation
from pptx.util import Inches, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE, MSO_CONNECTOR
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.oxml.ns import qn
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(SCRIPT_DIR, "pnc-2026-07-28-update.pptx")

prs = Presentation(); prs.slide_width = Inches(13.333); prs.slide_height = Inches(7.5)
BLACK=RGBColor(0x1A,0x1A,0x1A); GRAY=RGBColor(0x55,0x55,0x55); LGRAY=RGBColor(0x88,0x88,0x88)
DBLUE=RGBColor(0x00,0x3D,0x7A); MBLUE=RGBColor(0x2E,0x6D,0xB4); LBLUE=RGBColor(0xDCE7,0x00//1,0xF2) if False else RGBColor(0xDD,0xE9,0xF5)
GREEN=RGBColor(0x1E,0x7E,0x34); AMBER=RGBColor(0xB0,0x6A,0x00); WHITE=RGBColor(0xFF,0xFF,0xFF)
LGRAYBG=RGBColor(0xF5,0xF5,0xF5); TEAL=RGBColor(0x0F,0x76,0x74); PANEL=RGBColor(0xF7,0xF9,0xFC)

def slide():
    s=prs.slides.add_slide(prs.slide_layouts[6]); s.background.fill.solid()
    s.background.fill.fore_color.rgb=WHITE; return s
def title(s,t,top=0.4,size=30,color=BLACK):
    b=s.shapes.add_textbox(Inches(0.7),Inches(top),Inches(12),Inches(0.7)).text_frame.paragraphs[0]
    b.text=t; b.font.size=Pt(size); b.font.bold=True; b.font.color.rgb=color
def subtitle(s,t,top=1.05,size=15,color=GRAY):
    b=s.shapes.add_textbox(Inches(0.7),Inches(top),Inches(12),Inches(0.5)).text_frame.paragraphs[0]
    b.text=t; b.font.size=Pt(size); b.font.color.rgb=color
def accent(s,top=1.0,width=3.0):
    sh=s.shapes.add_shape(1,Inches(0.7),Inches(top),Inches(width),Pt(2))
    sh.fill.solid(); sh.fill.fore_color.rgb=DBLUE; sh.line.fill.background()
def footer(s,t="Akamai TLD Redirect Engine  —  PNC Domain Consolidation  —  Delivery update 2026-07-28"):
    p=s.shapes.add_textbox(Inches(0.7),Inches(7.12),Inches(12),Inches(0.3)).text_frame.paragraphs[0]
    p.text=t; p.font.size=Pt(9); p.font.color.rgb=LGRAY
def body(s,lines,top=1.7,left=0.7,width=11.9,size=15,ls=8,color=BLACK):
    tf=s.shapes.add_textbox(Inches(left),Inches(top),Inches(width),Inches(5)).text_frame; tf.word_wrap=True
    for i,l in enumerate(lines):
        p=tf.paragraphs[0] if i==0 else tf.add_paragraph()
        p.text=l; p.font.size=Pt(size); p.font.color.rgb=color; p.space_after=Pt(ls)

def box(s,x,y,w,h,text,fill=DBLUE,txt=WHITE,size=11,bold=True,line=None,radius=True):
    shp=s.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE if radius else MSO_SHAPE.RECTANGLE,
                           Inches(x),Inches(y),Inches(w),Inches(h))
    shp.fill.solid(); shp.fill.fore_color.rgb=fill
    if line: shp.line.color.rgb=line; shp.line.width=Pt(1)
    else: shp.line.fill.background()
    tf=shp.text_frame; tf.word_wrap=True; tf.vertical_anchor=MSO_ANCHOR.MIDDLE
    tf.margin_top=Pt(1); tf.margin_bottom=Pt(1)
    for i,ln in enumerate(text.split("\n")):
        p=tf.paragraphs[0] if i==0 else tf.add_paragraph()
        p.text=ln; p.alignment=PP_ALIGN.CENTER; p.font.size=Pt(size if i==0 else size-2)
        p.font.bold=bold if i==0 else False; p.font.color.rgb=txt
    return shp
def arrow(s,x1,y1,x2,y2,color=DBLUE,w=1.5,dash=False):
    c=s.shapes.add_connector(MSO_CONNECTOR.STRAIGHT,Inches(x1),Inches(y1),Inches(x2),Inches(y2))
    c.line.color.rgb=color; c.line.width=Pt(w)
    ln=c.line._get_or_add_ln()
    he=ln.makeelement(qn('a:tailEnd'),{'type':'triangle','w':'med','len':'med'}); ln.append(he)
    if dash:
        d=ln.makeelement(qn('a:prstDash'),{'val':'dash'}); ln.insert(0,d)
    return c
def label(s,x,y,w,t,size=8.5,color=GRAY,align=PP_ALIGN.CENTER,bold=False):
    p=s.shapes.add_textbox(Inches(x),Inches(y),Inches(w),Inches(0.3)).text_frame.paragraphs[0]
    p.text=t; p.font.size=Pt(size); p.font.color.rgb=color; p.alignment=align; p.font.bold=bold

# ============================ SLIDE 1 — Title ============================
s=slide()
title(s,"PNC Domain Consolidation",top=2.3,size=42)
subtitle(s,"Delivery update — follow-up to 2026-07-23 kickoff",top=3.15,size=23)
accent(s,top=3.05,width=3.5)
p=s.shapes.add_textbox(Inches(0.7),Inches(3.95),Inches(12),Inches(0.4)).text_frame.paragraphs[0]
p.text="2026-07-28  ·  environment live  ·  solution architecture  ·  live demo  ·  path to cutover"
p.font.size=Pt(15); p.font.color.rgb=GRAY
p=s.shapes.add_textbox(Inches(0.7),Inches(6.7),Inches(12),Inches(0.4)).text_frame.paragraphs[0]
p.text="Brian Apley · Technical Solutions Architect · bapley@akamai.com"
p.font.size=Pt(12); p.font.color.rgb=LGRAY

# ==================== SLIDE 2 — Since kickoff ====================
s=slide(); title(s,"Since kickoff — environment is live"); accent(s)
subtitle(s,"Stood up in PNC's compute environment and verified end-to-end")
rows=[
 ["Both regions provisioned","Done","us-ord + us-iad — 2 serving boxes + control + 3-node Managed PostgreSQL per region, all via Terraform."],
 ["Redirects serving","Verified","301s with path rules confirmed on both regions through their load balancers."],
 ["Cross-region sync","Verified","A rule created in one region appears in the other within ~10s (shared rule store)."],
 ["Admin TLS","Live","Real Let's Encrypt certs on both control planes — no DNS dependency for admin access."],
 ["Automatic failover","Built","Two serving boxes per region behind a health-checked load balancer."],
]
# simple table
from pptx.util import Inches as I
tbl=s.shapes.add_table(len(rows)+1,3,I(0.7),I(1.6),I(11.9),I(0.5)).table
tbl.columns[0].width=I(3.0); tbl.columns[1].width=I(1.3); tbl.columns[2].width=I(7.6)
for j,h in enumerate(["Milestone","Status","Detail"]):
    c=tbl.cell(0,j); c.text=h; c.fill.solid(); c.fill.fore_color.rgb=DBLUE
    for p in c.text_frame.paragraphs: p.font.size=Pt(13); p.font.bold=True; p.font.color.rgb=WHITE
for i,r in enumerate(rows):
    for j,v in enumerate(r):
        c=tbl.cell(i+1,j); c.text=v
        for p in c.text_frame.paragraphs:
            p.font.size=Pt(11); p.font.color.rgb=(GREEN if j==1 else BLACK); p.font.bold=(j==1)
        if i%2==1: c.fill.solid(); c.fill.fore_color.rgb=LGRAYBG
footer(s)

# ==================== SLIDE 3 — Architecture diagram ====================
s=slide(); title(s,"Solution architecture"); accent(s)
subtitle(s,"Two Linode regions · two serving boxes each · portable Reserved IP per region · PNC-owned DNS")

# top row: end users + DNS
box(s,0.5,1.65,1.7,0.6,"End users",fill=BLACK,size=12)
box(s,4.9,1.6,3.5,0.7,"PNC DNS  (customer-owned)\nA records → Reserved IPs",fill=MBLUE,size=12)
arrow(s,2.2,1.95,4.9,1.95)

# region panels
def region(x0,name):
    box(s,x0,2.55,4.7,4.15,"",fill=PANEL,line=RGBColor(0xC5,0xD3,0xE5),radius=True)
    label(s,x0,2.62,4.7,name,size=13,color=DBLUE,bold=True)
    cx=x0+0.1
    rip=box(s,cx+0.55,3.0,3.4,0.5,"Reserved IP  (portable)",fill=DBLUE,size=11)
    nb =box(s,cx+0.55,3.75,3.4,0.5,"NodeBalancer · TCP :80/:443",fill=DBLUE,size=11)
    sv1=box(s,cx+0.15,4.5,1.85,0.85,"Serving box 1\nTLS term · 301s",fill=MBLUE,size=10.5)
    sv2=box(s,cx+2.15,4.5,1.85,0.85,"Serving box 2\nTLS term · 301s",fill=MBLUE,size=10.5)
    ctl=box(s,cx+0.15,5.6,1.85,0.75,"Control plane\nadmin + certs",fill=GRAY,size=10.5)
    pg =box(s,cx+2.15,5.6,1.85,0.75,"Managed PG\n3-node",fill=GRAY,size=10.5)
    m=x0+2.45  # panel mid x
    arrow(s,m,3.5,m,3.75)             # rip -> nb
    arrow(s,m,4.25,cx+1.075,4.5)      # nb -> sv1
    arrow(s,m,4.25,cx+3.075,4.5)      # nb -> sv2
    return dict(rip_x=m,ctl_top=(cx+1.075,5.6),sv_bot=(m,5.35))
L=region(0.5,"us-ord  (Chicago)")
R=region(8.1,"us-iad  (Washington DC)")
# DNS -> reserved IPs
arrow(s,6.0,2.3,L["rip_x"],3.0)
arrow(s,7.3,2.3,R["rip_x"],3.0)

# Object Storage center
box(s,5.45,3.9,2.45,1.35,"Object Storage\nrules.json\ncross-region sync\n(poll ~5s)",fill=TEAL,size=11)
# control -> objst (write), objst -> serving (poll)
arrow(s,L["ctl_top"][0]+0.9,5.6,5.6,4.9,color=TEAL,w=1.25)          # ord control -> objst
arrow(s,R["ctl_top"][0]+0.0,5.6,7.75,4.9,color=TEAL,w=1.25)          # iad control -> objst
label(s,5.2,5.25,3.0,"control writes  •  serving boxes poll",size=8,color=TEAL)

# end-users flow note
label(s,0.3,2.35,2.2,"resolve → Reserved IP",size=8,color=GRAY,align=PP_ALIGN.LEFT)
footer(s)

# ==================== SLIDE 4 — Cert workflow ====================
s=slide(); title(s,"TLS / certificate workflow"); accent(s)
subtitle(s,"Automatic per-domain certificates — no 10-cert load-balancer limit, scales to the full inventory")
steps=[
 ("1","PNC points DNS","A record for a redirect domain → the region's Reserved IP"),
 ("2","First request arrives","Serving box sees a new domain with no cert yet"),
 ("3","ACME challenge","Let's Encrypt HTTP-01 on :80 — solved on the serving box"),
 ("4","Cert issued + stored","Shared cert store; every serving box serves TLS for that domain"),
]
x=0.7; w=2.85; gap=0.28
for i,(n,t,d) in enumerate(steps):
    xi=x+i*(w+gap)
    box(s,xi,2.2,w,1.7,f"{n}",fill=DBLUE,size=22)
    lb=s.shapes.add_textbox(Inches(xi),Inches(2.55),Inches(w),Inches(1.3)).text_frame; lb.word_wrap=True
    p=lb.paragraphs[0]; p.text=t; p.alignment=PP_ALIGN.CENTER; p.font.size=Pt(13); p.font.bold=True; p.font.color.rgb=WHITE
    p2=lb.add_paragraph(); p2.text=d; p2.alignment=PP_ALIGN.CENTER; p2.font.size=Pt(10); p2.font.color.rgb=RGBColor(0xE8,0xEE,0xF6)
    if i<3: arrow(s,xi+w,3.05,xi+w+gap,3.05)
body(s,[
 "•  Load balancer runs TCP passthrough, so TLS terminates on the serving boxes — removes the 10-certificate",
 "    limit and keeps certificate management in the application, provisioned on demand for 2000+ domains.",
 "•  Renewals are automatic. Admin control planes already run real Let's Encrypt certs today (via their",
 "    Linode hostnames), so the workflow is proven — redirect-domain certs issue the moment PNC points DNS.",
], top=4.5,size=13,ls=6)
footer(s)

# ==================== SLIDE 5 — Live demo + status ====================
s=slide(); title(s,"Live demo  ·  what's live vs. in progress"); accent(s)
subtitle(s,"Admin console is reachable now — demoing domain CRUD, bulk import, and cross-region sync")
box(s,0.7,1.6,11.9,0.7,"Control plane (ORD):   https://172-233-217-89.ip.linodeusercontent.com/ui/",
    fill=RGBColor(0x11,0x2A,0x45),size=13)
rows=[
 ["Compute, load balancers, PostgreSQL","Live","Provisioned in PNC's account, both regions."],
 ["Redirect serving + cross-region sync","Live","Verified end-to-end on both regions."],
 ["Admin console + admin TLS","Live","Real Let's Encrypt cert; access locked to approved IPs."],
 ["Portable Reserved IPs (DNS targets)","In progress","Needs a networking/IP role on the delivery account."],
 ["Rule store in PNC's Object Storage","In progress","Needs an Object Storage role; on interim storage today."],
 ["Redirect-domain TLS + observability","Pending PNC","Certs issue when DNS is pointed; sink to be confirmed."],
]
tbl=s.shapes.add_table(len(rows)+1,3,I(0.7),I(2.5),I(11.9),I(0.46)).table
tbl.columns[0].width=I(4.4); tbl.columns[1].width=I(1.9); tbl.columns[2].width=I(5.6)
for j,h in enumerate(["Component","Status","Note"]):
    c=tbl.cell(0,j); c.text=h; c.fill.solid(); c.fill.fore_color.rgb=DBLUE
    for p in c.text_frame.paragraphs: p.font.size=Pt(12); p.font.bold=True; p.font.color.rgb=WHITE
for i,r in enumerate(rows):
    for j,v in enumerate(r):
        c=tbl.cell(i+1,j); c.text=v
        col=BLACK
        if j==1: col=GREEN if v=="Live" else (AMBER if v=="In progress" else GRAY)
        for p in c.text_frame.paragraphs: p.font.size=Pt(10.5); p.font.color.rgb=col; p.font.bold=(j==1)
        if i%2==1: c.fill.solid(); c.fill.fore_color.rgb=LGRAYBG
footer(s)

# ==================== SLIDE 6 — Next steps / asks ====================
s=slide(); title(s,"Next steps — path to cutover"); accent(s)
subtitle(s,"Two quick account-permission grants unblock the permanent configuration")
rows=[
 ["1","Grant Object Storage role on the delivery account","PNC","Moves the rule store into PNC's own Object Storage. One IAM role."],
 ["2","Grant networking/IP role on the delivery account","PNC","Lets us assign the portable Reserved IPs — your stable DNS targets."],
 ["3","Point test DNS at the two Reserved IPs","PNC","Validates real redirect domains + auto-TLS end-to-end before cutover."],
 ["4","Provide the redirect inventory (domain → target → code)","PNC","Bulk-imported; this is the data that makes the redirects real."],
 ["5","Confirm the observability sink","PNC","DataStream 2 or a PNC-preferred target for redirect telemetry."],
 ["6","F5 cutover — dry-run, then migrate DNS in batches","Joint","Rollback at each step; handoff to PNC operations."],
]
tbl=s.shapes.add_table(len(rows)+1,4,I(0.7),I(1.65),I(11.9),I(0.5)).table
for i,wd in enumerate([0.4,5.3,1.2,5.0]): tbl.columns[i].width=I(wd)
for j,h in enumerate(["#","Item","Owner","Why / outcome"]):
    c=tbl.cell(0,j); c.text=h; c.fill.solid(); c.fill.fore_color.rgb=DBLUE
    for p in c.text_frame.paragraphs: p.font.size=Pt(12); p.font.bold=True; p.font.color.rgb=WHITE
for i,r in enumerate(rows):
    for j,v in enumerate(r):
        c=tbl.cell(i+1,j); c.text=v
        for p in c.text_frame.paragraphs: p.font.size=Pt(10.5); p.font.color.rgb=BLACK
        if i%2==1: c.fill.solid(); c.fill.fore_color.rgb=LGRAYBG
p=s.shapes.add_textbox(Inches(0.7),Inches(6.35),Inches(12),Inches(0.5)).text_frame.paragraphs[0]
p.text="Today's asks:  the two account-permission grants (#1, #2) — everything permanent flows from those."
p.font.size=Pt(13); p.font.bold=True; p.font.color.rgb=DBLUE
footer(s)

prs.save(OUT); print("Wrote:",OUT,"| slides:",len(prs.slides))
