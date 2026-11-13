#!/usr/bin/env python3
"""Mutant table for scripts/mutation_g1_round3.sh.
  mutants.py count        -> number of mutants
  mutants.py file N       -> file the Nth mutant edits
  mutants.py apply N      -> apply the Nth mutant (exit 3 if the pattern does not match exactly once)
"""
import sys
M=[
# (id, file, old, new, pkgs, runfilter)
("M01 lone - accepted","pkg/runtime/run_policy.go",'case strings.HasPrefix(a, "-") && len(a) > 1:','case strings.HasPrefix(a, "-") && len(a) > 0:'),
("M02 unknown flag accepted","pkg/runtime/run_policy.go",'return forbidden("extra arg %q is not on the flag allowlist", a)','continue'),
("M03 bool flag takes value","pkg/runtime/run_policy.go",'if hasVal {\n\t\t\t\t\treturn forbidden("extra arg %q: boolean flags take no value", a)','if false {\n\t\t\t\t\treturn forbidden("extra arg %q: boolean flags take no value", a)'),
("M04 stray positional accepted","pkg/runtime/run_policy.go",'return forbidden("extra arg %q is a positional, not a flag", a)','continue'),
("M05 unknown short flag accepted","pkg/runtime/run_policy.go",'return forbidden("extra arg %q: short flag -%c is not on the allowlist", a, a[j])','continue'),
("M06 cluster value does not end cluster","pkg/runtime/run_policy.go",'\t\t\t\t\treturn fmt.Errorf("%s: %w", long, err)\n\t\t\t\t}\n\t\t\t\tbreak','\t\t\t\t\treturn fmt.Errorf("%s: %w", long, err)\n\t\t\t\t}\n\t\t\t\tcontinue'),
("M07 env without = accepted","pkg/runtime/run_policy.go",'if !ok || !reEnvName.MatchString(k) {','if (!ok && false) || !reEnvName.MatchString(k) {'),
("M08 abs path not cleaned","pkg/runtime/run_policy.go",'if !path.IsAbs(v) || path.Clean(v) != v ||','if !path.IsAbs(v) ||'),
("M09 container /proc target allowed","pkg/runtime/run_policy.go",'t == "/" || t == "/proc" || strings.HasPrefix(t, "/proc/") ||','t == "/" ||'),
("M10 network host allowed","pkg/runtime/run_policy.go",'case l == "host" || l == "container"','case l == "hostX" || l == "container"'),
("M11 network name regex dropped","pkg/runtime/run_policy.go",'case !reName.MatchString(n):','case false:'),
("M12 /home exact not denied","pkg/runtime/run_policy.go",'if p == "/" || p == "/home" || p == "/tmp" {','if p == "/" || p == "/tmp" {'),
("M13 user home dir not denied","pkg/runtime/run_policy.go",'strings.Count(p[1:], "/") == 1','strings.Count(p[1:], "/") == 5'),
("M14 /usr not denied","pkg/runtime/run_policy.go",'"/root", "/usr",\n','"/root",\n'),
("M15 /var not denied","pkg/runtime/run_policy.go",'"/run", "/var", "/etc",','"/run", "/etc",'),
("M16 /etc not denied","pkg/runtime/run_policy.go",'"/var", "/etc", "/boot",','"/var", "/boot",'),
("M17 .sock name not denied","pkg/runtime/run_policy.go",'strings.HasSuffix(p, ".sock") ||','false ||'),
("M18 home credential dirs not denied","pkg/runtime/run_policy.go",'".ssh", ".gnupg",','".gnupgX",'),
("M19 allowed-root check bypassed","pkg/runtime/run_policy.go",'if within(use, r) {','if within(use, r) || true {'),
("M20 symlinks not resolved","pkg/runtime/run_policy.go",'\tif !o.remote {\n\t\tr, err := filepath.EvalSymlinks(src)','\tif false {\n\t\tr, err := filepath.EvalSymlinks(src)'),
("M21 resolved path not used in argv","pkg/runtime/run_policy.go",'use = r // pass','_ = r // pass'),
("M22 resolved path not denied","pkg/runtime/run_policy.go",'cands = append(cands, r)\n\t\tuse = r','use = r'),
("M23 socket mode not checked","pkg/runtime/run_policy.go",'fi.Mode()&os.ModeSocket != 0','fi.Mode()&os.ModeSocket != 0 && false'),
("M24 root itself not within","pkg/runtime/run_policy.go",'func within(p, root string) bool { return p == root ||','func within(p, root string) bool { return false ||'),
("M25 sibling-prefix within","pkg/runtime/run_policy.go",'strings.HasPrefix(p, root+"/") }','strings.HasPrefix(p, root) }'),
("M26 unknown mount type passes through","pkg/runtime/run_policy.go",'return forbidden("mount type %q is not available (bind, volume, tmpfs only)", m.Type)','kv = []string{"type=" + string(m.Type), "source=" + m.Source, "target=" + m.Target}'),
("M27 readonly dropped","pkg/runtime/run_policy.go",'if m.ReadOnly {\n\t\t\tkv = append(kv, "readonly")','if false {\n\t\t\tkv = append(kv, "readonly")'),
("M28 volume option Z accepted","pkg/runtime/run_policy.go",'\t\t\tcase "rw":\n','\t\t\tcase "rw", "Z":\n'),
("M29 NET_ADMIN without reason","pkg/runtime/run_policy.go",'\t\t\tif !reasoned {','\t\t\tif !reasoned && false {'),
("M30 MKNOD in safe caps","pkg/runtime/run_policy.go",'"CHOWN": true,','"CHOWN": true, "MKNOD": true,'),
("M31 any cap accepted","pkg/runtime/run_policy.go",'return forbidden("capability %q is not on the capability allowlist", c)','seen[n] = true'),
("M32 short reason accepted","pkg/runtime/run_policy.go",'len(strings.TrimSpace(r)) < 8','len(strings.TrimSpace(r)) < 0'),
("M33 --privileged on allowlist","pkg/runtime/run_policy.go",'"--read-only":    {},','"--read-only":    {},\n\t"--privileged": {},'),
("M34 --security-opt on allowlist","pkg/runtime/run_policy.go",'"--read-only":    {},','"--read-only":    {},\n\t"--security-opt": {value: true, validate: vLabel},'),
("M35 ExtraArgs not scanned","pkg/runtime/run_policy.go",'if err := scanExtraArgs(o.ExtraArgs); err != nil {','if err := error(nil); err != nil {'),
("M36 mounts not built","pkg/runtime/run_policy.go",'if err := o.buildMounts(); err != nil {','if err := error(nil); err != nil {'),
("M37 ResolveRunSpec resolves locally","pkg/runtime/run.go",'o.remote = true //','o.remote = false //'),
("M38 tmpfs suid accepted","pkg/runtime/run_policy.go",'noexec|exec|nosuid|','noexec|exec|suid|nosuid|'),
("M39 ulimit negative accepted","pkg/runtime/run_policy.go",'strconv.ParseUint(p, 10, 32)','strconv.ParseInt(p, 10, 32)'),
("M40 label empty key accepted","pkg/runtime/run_policy.go",'if !ok || k == "" ||','if !ok ||'),
("M41 container name unchecked","pkg/runtime/run_policy.go",'if o.Name != "" && !reName.MatchString(o.Name) {','if false {'),
("M42 no-new-privileges not emitted","pkg/runtime/run.go",'if o.NoNewPrivs {','if false {'),
("M43 cap-drop ALL not emitted","pkg/runtime/run.go",'if o.CapDropAll {','if false {'),
("M44 NaN cpus accepted","pkg/hostbudget/hostbudget.go",'!(need.CPUs >= 0) ||','need.CPUs < 0 ||'),
("M45 Inf cpus accepted","pkg/hostbudget/hostbudget.go",'|| math.IsInf(need.CPUs, 0) || need.Pids < 0 {','|| need.Pids < 0 {'),
("M46 negative pids accepted","pkg/hostbudget/hostbudget.go",'|| need.Pids < 0 {\n\t\tev.Verdict','{\n\t\tev.Verdict'),
("M47 unreadable status skipped","pkg/hostbudget/hostbudget.go",'if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {','if true {'),
("M48 unparsable status skipped","pkg/hostbudget/hostbudget.go",'return 0, fmt.Errorf("pid %s status has no Uid/Threads", e.Name())','continue'),
("M49 fsync failure ignored","pkg/hostbudget/hostbudget.go",'if err := syncFile(f); err != nil {','if err := error(nil); err != nil {'),
("M50 partial file kept","pkg/hostbudget/hostbudget.go",'_ = f.Close()\n\t\t_ = os.Remove(p)\n\t\treturn "", err','_ = f.Close()\n\t\treturn "", err'),
("M51 ADB rc 126/127 not could-not-run","pkg/emulator/reachable.go",'if rc == 126 || rc == 127 {','if false {'),
("M52 ADB rc 127 not could-not-run","pkg/emulator/reachable.go",'rc == 126 || rc == 127','rc == 126'),
('M53 symlinked root not re-denied','pkg/runtime/run_policy.go','\t\t\tr = rr\n\t\t\tif d, why := denyHostPath(r, home); d {','\t\t\tr = rr\n\t\t\tif d, why := denyHostPath(r, home); d && false {'),
('M54 container target comma/quote allowed','pkg/runtime/run_policy.go','strings.ContainsAny(t, ",\\"")','strings.ContainsAny(t, "")'),
('M55 bind source comma allowed','pkg/runtime/run_policy.go','strings.ContainsAny(src, ",\\"\\n\\r\\x00")','strings.ContainsAny(src, "\\"\\n\\r\\x00")'),
('M56 bind source quote allowed','pkg/runtime/run_policy.go','strings.ContainsAny(src, ",\\"\\n\\r\\x00")','strings.ContainsAny(src, ",\\n\\r\\x00")'),
('M57 bind source newline allowed','pkg/runtime/run_policy.go','strings.ContainsAny(src, ",\\"\\n\\r\\x00")','strings.ContainsAny(src, ",\\"\\r\\x00")'),
('M58 add-host hostname unchecked','pkg/runtime/run_policy.go','!reHostname.MatchString(h) ||','false ||'),
('M59 ulimit name unchecked','pkg/runtime/run_policy.go','if !ok || !ulimitOK[name] {','if !ok {'),
('M60 ESRCH not skipped','pkg/hostbudget/hostbudget.go',' || errors.Is(err, syscall.ESRCH) {',' {'),
('M61 stop-signal number unbounded','pkg/runtime/run_policy.go','if n >= 1 && n <= 64 {','if true {'),
('M62 detach not emitted','pkg/runtime/run.go','if o.Detach {','if false {'),
('M63 pids upper bound dropped','pkg/runtime/run.go','o.PidsLimit <= 0 || o.PidsLimit > maxPidsLimit','o.PidsLimit <= 0'),
('M64 memory lower bound dropped','pkg/runtime/run.go','o.memorySet && o.Memory < minMemoryBytes','false'),
('M65 reason not tied to capability','pkg/runtime/run_policy.go','if !o.reasonedCaps[n] {','if false {'),
('M66 .bashrc not denied','pkg/runtime/run_policy.go','".bashrc", ".bash_profile",','".bash_profile",'),
('M67 /mnt not denied','pkg/runtime/run_policy.go','"/srv", "/mnt", "/media",','"/srv", "/media",'),
('M68 memory unit k wrong','pkg/runtime/run_policy.go','"k": 1 << 10,','"k": 1 << 11,'),
('M69 cpus bound dropped','pkg/runtime/run.go','o.CPUs > maxCPUs || math.IsNaN(o.CPUs)','math.IsNaN(o.CPUs)'),
]
cmd=sys.argv[1]
if cmd=="count":
    print(len(M))
else:
    i=int(sys.argv[2]); mid,f,old,new=M[i]
    if cmd=="file":
        print(f)
    else:
        s=open(f).read()
        if s.count(old)!=1:
            print("NOMATCH(%d) %s"%(s.count(old),mid)); sys.exit(3)
        open(f,"w").write(s.replace(old,new)); print(mid)
