package lsp

// Member help catalog for hover, completion docs, and signature help.
// Keep entries short and honest — prefer weft stdlib over inventing APIs.

type memberHelp struct {
	Sig    string // e.g. llm.ask(prompt, tools?, opts?) -> Result[str]
	Detail string // one-line prose
}

// memberCatalog is keyed as "pkg.member" or bare prelude names.
var memberCatalog = map[string]memberHelp{
	// prelude
	"map":        {Sig: "map(list, fn, workers?)", Detail: "concurrent map; order preserved"},
	"seq_map":    {Sig: "seq_map(list, fn)", Detail: "sequential map"},
	"filter":     {Sig: "filter(list, pred)", Detail: "concurrent filter"},
	"seq_filter": {Sig: "seq_filter(list, pred)", Detail: "sequential filter"},
	"gather":     {Sig: "gather([fn…]) -> Result", Detail: "concurrent fan-out"},
	"parallel":   {Sig: "parallel([fn…]) -> Result", Detail: "same as gather"},
	"race":       {Sig: "race([fn…]) -> Result", Detail: "first completed wins"},
	"timeout":    {Sig: "timeout(seconds, fn) -> Result", Detail: "deadline wrapper"},
	"spawn":      {Sig: "spawn(fn, args…)", Detail: "background task; .await()?"},
	"ensure":     {Sig: "ensure(cond, msg?, kind?) -> Result", Detail: "precondition; use with ?"},
	"bail":       {Sig: "bail(msg, kind?) -> Result", Detail: "early Err Result"},
	"say":        {Sig: "say(values…)", Detail: "print line"},
	"println":    {Sig: "println(values…)", Detail: "print line (fmt rewrites to say)"},
	"channel":    {Sig: "channel(cap?)", Detail: "buffered channel"},
	"send":       {Sig: "send(ch, v) -> Result", Detail: "send on channel"},
	"recv":       {Sig: "recv(ch) -> Result", Detail: "receive (blocks)"},
	"try_recv":   {Sig: "try_recv(ch) -> Result", Detail: "non-blocking receive {ok, value}"},
	"close":      {Sig: "close(ch) -> Result", Detail: "close channel"},
	"group":      {Sig: "group() -> task group", Detail: ".go(fn) / .wait()?"},
	"len":        {Sig: "len(x) -> int", Detail: "length of list/str/map"},
	"push":       {Sig: "push(list, v)", Detail: "append to list"},
	"range":      {Sig: "range(n | start, end)", Detail: "numeric sequence"},
	"Ok":         {Sig: "Ok(value) -> Result", Detail: "success Result"},
	"Err":        {Sig: "Err(msg, kind?) -> Result", Detail: "failure Result"},

	// llm
	"llm.chat": {
		Sig:    "llm.chat(prompt | messages | opts) -> Result[str]",
		Detail: "one-shot or multi-turn; opts: system, model, …",
	},
	"llm.ask": {
		Sig:    "llm.ask(prompt, tools?, opts?) -> Result[str]",
		Detail: "tool-using agent; opts: system, max_steps, model",
	},
	"llm.agent": {
		Sig:    "llm.agent(tools | {tools, …}) -> agent",
		Detail: "reusable agent; call .run(prompt)",
	},
	"llm.tool": {
		Sig:    "llm.tool(name, fn, desc?)",
		Detail: "bind a Weft fn as a model tool",
	},
	"llm.stream": {
		Sig:    "llm.stream(prompt | opts) -> Result[Iter]",
		Detail: "SSE events {kind, text?}: text|done|error",
	},
	"llm.stream_text": {
		Sig:    "llm.stream_text(prompt | opts) -> Result[str]",
		Detail: "collect stream text events into one string",
	},
	"llm.extract": {
		Sig:    "llm.extract(prompt | opts) -> Result[map]",
		Detail: "JSON object from the model",
	},
	"llm.client": {
		Sig:    "llm.client(opts?) -> {chat, agent}",
		Detail: "bound client with fixed opts",
	},

	// http
	"http.get":       {Sig: "http.get(url, opts?) -> Result", Detail: "HTTP GET → {status, body, headers, ok}"},
	"http.get_json":  {Sig: "http.get_json(url, opts?) -> Result", Detail: "GET + parse JSON body"},
	"http.post":      {Sig: "http.post(url, body?, opts?) -> Result", Detail: "HTTP POST (JSON by default)"},
	"http.put":       {Sig: "http.put(url, body?, opts?) -> Result", Detail: "HTTP PUT"},
	"http.patch":     {Sig: "http.patch(url, body?, opts?) -> Result", Detail: "HTTP PATCH"},
	"http.delete":    {Sig: "http.delete(url, opts?) -> Result", Detail: "HTTP DELETE"},
	"http.fetch":     {Sig: "http.fetch(opts) -> Result", Detail: "generic request map"},
	"http.post_form": {Sig: "http.post_form(url, form, opts?) -> Result", Detail: "multipart/form POST"},
	"http.serve":     {Sig: "http.serve(addr, handler)", Detail: "tiny HTTP server (blocking)"},
	"http.json":      {Sig: "http.json(body | status, body)", Detail: "JSON response for serve handlers"},
	"http.text":      {Sig: "http.text(status, body)", Detail: "text response for serve handlers"},

	// web
	"web.app":           {Sig: "web.app() -> app", Detail: "multi-route app; .get/.post/.listen"},
	"web.json":          {Sig: "web.json(v)", Detail: "JSON response helper"},
	"web.html":          {Sig: "web.html(s)", Detail: "HTML response helper"},
	"web.text":          {Sig: "web.text(s)", Detail: "plain text response"},
	"web.redirect":      {Sig: "web.redirect(url, status?)", Detail: "redirect response"},
	"web.sse":           {Sig: "web.sse(list | iter)", Detail: "Server-Sent Events stream"},
	"web.status":        {Sig: "web.status(code, body?)", Detail: "status response"},
	"web.is_htmx":       {Sig: "web.is_htmx(req) -> bool", Detail: "true when HX-Request header set"},
	"web.htmx":          {Sig: "web.htmx(html, opts?)", Detail: "HTML partial + HX-* response headers"},
	"web.htmx_redirect": {Sig: "web.htmx_redirect(url)", Detail: "HX-Redirect client navigation"},
	"web.htmx_refresh":  {Sig: "web.htmx_refresh()", Detail: "HX-Refresh: true"},
	"web.htmx_trigger":  {Sig: "web.htmx_trigger(event|map, html?)", Detail: "HX-Trigger header"},
	"web.htmx_location": {Sig: "web.htmx_location(url|opts)", Detail: "HX-Location soft nav"},
	"web.htmx_cdn":      {Sig: "web.htmx_cdn(version?)", Detail: "script tag for htmx CDN"},
	"web.form":          {Sig: "web.form(req) -> map", Detail: "parsed form fields (query + body)"},
	"web.form_get":      {Sig: "web.form_get(req, key, default?)", Detail: "one form field as string"},
	"web.form_list":     {Sig: "web.form_list(req, key) -> [str]", Detail: "all values for multi-select fields"},
	"web.file":          {Sig: "web.file(req, field) -> map|null", Detail: "multipart file {filename,body,size,…}"},
	"web.htmx_oob":      {Sig: "web.htmx_oob(id, html)", Detail: "hx-swap-oob fragment"},
	"web.cookie":        {Sig: "web.cookie(name, value, opts?)", Detail: "Set-Cookie string for responses"},
	"web.clear_cookie":  {Sig: "web.clear_cookie(name, opts?)", Detail: "expire a cookie"},
	"web.cookie_get":    {Sig: "web.cookie_get(req, name, default?)", Detail: "read request cookie"},

	// fs
	"fs.read":      {Sig: "fs.read(path) -> Result[str]", Detail: "read whole file"},
	"fs.write":     {Sig: "fs.write(path, data) -> Result", Detail: "write file"},
	"fs.append":    {Sig: "fs.append(path, data) -> Result", Detail: "append to file"},
	"fs.exists":    {Sig: "fs.exists(path) -> bool", Detail: "path exists"},
	"fs.lines":     {Sig: "fs.lines(path) -> Result[[str]]", Detail: "read lines"},
	"fs.glob":      {Sig: "fs.glob(pattern) -> [str]", Detail: "glob paths"},
	"fs.rglob":     {Sig: "fs.rglob(pattern) -> [str]", Detail: "recursive glob"},
	"fs.join":      {Sig: "fs.join(parts…)", Detail: "join path segments"},
	"fs.base":      {Sig: "fs.base(path)", Detail: "base name"},
	"fs.dir":       {Sig: "fs.dir(path)", Detail: "directory of path"},
	"fs.ext":       {Sig: "fs.ext(path)", Detail: "extension"},
	"fs.cwd":       {Sig: "fs.cwd()", Detail: "current working directory"},
	"fs.abs":       {Sig: "fs.abs(path)", Detail: "absolute path"},
	"fs.list":      {Sig: "fs.list(dir) -> Result", Detail: "list directory"},
	"fs.mkdir":     {Sig: "fs.mkdir(path) -> Result", Detail: "create directory"},
	"fs.remove":    {Sig: "fs.remove(path) -> Result", Detail: "remove file"},
	"fs.temp_file": {Sig: "fs.temp_file(prefix?, suffix?) -> Result[str]", Detail: "temp file path"},
	"fs.temp_dir":  {Sig: "fs.temp_dir(prefix?) -> Result[str]", Detail: "temp directory"},
	"fs.walk":      {Sig: "fs.walk(root) -> Result", Detail: "walk tree"},
	"fs.stat":      {Sig: "fs.stat(path) -> Result", Detail: "file info"},
	"fs.size":      {Sig: "fs.size(path) -> Result[int]", Detail: "file size bytes"},

	// json / jsonl / config
	"json.parse":     {Sig: "json.parse(s) -> Result", Detail: "parse JSON text"},
	"json.stringify": {Sig: "json.stringify(v, indent?)", Detail: "serialize to JSON"},
	"json.pretty":    {Sig: "json.pretty(v)", Detail: "pretty-print JSON"},
	"json.get":       {Sig: "json.get(doc, path, default?) -> Result", Detail: "dotted path; default if missing"},
	"json.set":       {Sig: "json.set(doc, path, v)", Detail: "set dotted path"},
	"json.has":       {Sig: "json.has(doc, path) -> bool", Detail: "path exists"},
	"json.merge":     {Sig: "json.merge(a, b)", Detail: "shallow/deep merge helpers"},
	"json.clone":     {Sig: "json.clone(v)", Detail: "deep copy"},
	"jsonl.read":     {Sig: "jsonl.read(path) -> Result", Detail: "read JSON Lines"},
	"jsonl.write":    {Sig: "jsonl.write(path, rows) -> Result", Detail: "write JSON Lines"},
	"jsonl.append":   {Sig: "jsonl.append(path, row) -> Result", Detail: "append one line"},
	"jsonl.parse":    {Sig: "jsonl.parse(s) -> Result", Detail: "parse JSONL text"},
	"yaml.parse":     {Sig: "yaml.parse(s) -> Result", Detail: "parse YAML"},
	"yaml.load":      {Sig: "yaml.load(path) -> Result", Detail: "load YAML file"},
	"yaml.stringify": {Sig: "yaml.stringify(v)", Detail: "YAML text"},
	"yaml.save":      {Sig: "yaml.save(path, v) -> Result", Detail: "write YAML file"},
	"toml.parse":     {Sig: "toml.parse(s) -> Result", Detail: "parse TOML"},
	"toml.load":      {Sig: "toml.load(path) -> Result", Detail: "load TOML file"},
	"toml.stringify": {Sig: "toml.stringify(v)", Detail: "TOML text"},
	"toml.save":      {Sig: "toml.save(path, v) -> Result", Detail: "write TOML file"},
	"ini.parse":      {Sig: "ini.parse(s) -> Result", Detail: "parse INI"},
	"ini.load":       {Sig: "ini.load(path) -> Result", Detail: "load INI file"},
	"ini.save":       {Sig: "ini.save(path, v) -> Result", Detail: "write INI file"},
	"ini.get":        {Sig: "ini.get(doc, section, key)", Detail: "section key lookup"},
	"xml.parse":      {Sig: "xml.parse(s) -> Result", Detail: "parse XML"},
	"xml.stringify":  {Sig: "xml.stringify(v)", Detail: "XML text"},

	// db / data
	"db.open":            {Sig: "db.open(dsn) -> Result[conn]", Detail: "sqlite:… / postgres / mysql"},
	"db.drivers":         {Sig: "db.drivers()", Detail: "available drivers"},
	"csv.parse":          {Sig: "csv.parse(s, opts?) -> Result", Detail: "parse CSV text"},
	"csv.read":           {Sig: "csv.read(path, opts?) -> Result", Detail: "read CSV file"},
	"csv.write":          {Sig: "csv.write(path, rows, opts?) -> Result", Detail: "write CSV"},
	"csv.stringify":      {Sig: "csv.stringify(rows, opts?)", Detail: "CSV text"},
	"table.where_eq":     {Sig: "table.where_eq(rows, col, val)", Detail: "filter rows"},
	"table.where_ne":     {Sig: "table.where_ne(rows, col, val)", Detail: "filter != value"},
	"table.where_truthy": {Sig: "table.where_truthy(rows, col)", Detail: "truthy column filter"},
	"table.project":      {Sig: "table.project(rows, cols)", Detail: "keep columns"},
	"table.pick":         {Sig: "table.pick(rows, cols)", Detail: "pick columns"},
	"table.pluck":        {Sig: "table.pluck(rows, col)", Detail: "column values"},
	"table.sort":         {Sig: "table.sort(rows, col, desc?)", Detail: "sort rows"},
	"table.unique":       {Sig: "table.unique(rows, col?)", Detail: "unique rows/values"},
	"table.group_by":     {Sig: "table.group_by(rows, col)", Detail: "group rows"},
	"table.merge":        {Sig: "table.merge(a, b, on)", Detail: "join-like merge"},
	"table.count":        {Sig: "table.count(rows)", Detail: "row count"},

	// cli / env / log / secrets / sh / io
	"cli.parse":       {Sig: "cli.parse(spec) -> Result", Detail: "flags/args; .help .usage .args"},
	"cli.exit":        {Sig: "cli.exit(code)", Detail: "exit process"},
	"cli.argv":        {Sig: "cli.argv() -> [str]", Detail: "positionals after script name"},
	"cli.args":        {Sig: "cli.args() -> [str]", Detail: "full argv"},
	"cli.die":         {Sig: "cli.die(msg)", Detail: "print error and exit"},
	"cli.flag":        {Sig: "cli.flag(name, default)", Detail: "quick flag lookup"},
	"env.get":         {Sig: "env.get(name, default?) -> any", Detail: "env var or default/null"},
	"env.require":     {Sig: "env.require(name) -> Result[str]", Detail: "required env var"},
	"env.set":         {Sig: "env.set(name, value)", Detail: "set env var"},
	"env.keys":        {Sig: "env.keys() -> [str]", Detail: "env names"},
	"env.home":        {Sig: "env.home()", Detail: "home directory"},
	"env.hostname":    {Sig: "env.hostname()", Detail: "host name"},
	"env.pid":         {Sig: "env.pid()", Detail: "process id"},
	"log.info":        {Sig: "log.info(msg, fields?)", Detail: "info log"},
	"log.warn":        {Sig: "log.warn(msg, fields?)", Detail: "warn log"},
	"log.error":       {Sig: "log.error(msg, fields?)", Detail: "error log"},
	"log.debug":       {Sig: "log.debug(msg, fields?)", Detail: "debug log"},
	"log.set_level":   {Sig: "log.set_level(level)", Detail: "debug|info|warn|error"},
	"secrets.get":     {Sig: "secrets.get(name)", Detail: "secret value if set"},
	"secrets.require": {Sig: "secrets.require(name) -> Result", Detail: "required secret"},
	"secrets.from":    {Sig: "secrets.from(map)", Detail: "secret bag helper"},
	"sh.run":          {Sig: "sh.run(cmd, args?) -> Result", Detail: "run process"},
	"sh.capture":      {Sig: "sh.capture(cmd, args?) -> Result[str]", Detail: "stdout as string"},
	"sh.shell":        {Sig: "sh.shell(line) -> Result", Detail: "run via shell"},
	"sh.which":        {Sig: "sh.which(bin)", Detail: "resolve binary path"},
	"sh.ok":           {Sig: "sh.ok(cmd, args?) -> bool", Detail: "exit 0?"},
	"io.stdin":        {Sig: "io.stdin() -> Result[str]", Detail: "read stdin"},
	"io.lines":        {Sig: "io.lines() -> Result", Detail: "stdin lines"},
	"io.eprintln":     {Sig: "io.eprintln(values…)", Detail: "print to stderr"},

	// time / str / re / math
	"time.now":        {Sig: "time.now() -> int", Detail: "unix seconds"},
	"time.now_ms":     {Sig: "time.now_ms() -> int", Detail: "unix milliseconds"},
	"time.iso":        {Sig: "time.iso(t?)", Detail: "ISO-8601 string"},
	"time.parse":      {Sig: "time.parse(s, layout?) -> Result", Detail: "parse time"},
	"time.format":     {Sig: "time.format(t, layout)", Detail: "format time"},
	"time.parts":      {Sig: "time.parts(t) -> map", Detail: "year/month/day/…"},
	"time.from_parts": {Sig: "time.from_parts(y, m, d, …)", Detail: "build unix time"},
	"time.sleep":      {Sig: "time.sleep(seconds)", Detail: "sleep"},
	"time.sleep_ms":   {Sig: "time.sleep_ms(ms)", Detail: "sleep milliseconds"},
	"str.trim":        {Sig: "str.trim(s)", Detail: "trim whitespace"},
	"str.split":       {Sig: "str.split(s, sep)", Detail: "split string"},
	"str.join":        {Sig: "str.join(parts, sep)", Detail: "join list of strings"},
	"str.lower":       {Sig: "str.lower(s)", Detail: "lowercase"},
	"str.upper":       {Sig: "str.upper(s)", Detail: "uppercase"},
	"str.contains":    {Sig: "str.contains(s, sub) -> bool", Detail: "substring check"},
	"str.replace":     {Sig: "str.replace(s, old, new)", Detail: "replace all"},
	"str.has_prefix":  {Sig: "str.has_prefix(s, pfx)", Detail: "prefix check"},
	"str.has_suffix":  {Sig: "str.has_suffix(s, sfx)", Detail: "suffix check"},
	"str.starts_with": {Sig: "str.starts_with(s, pfx)", Detail: "alias of has_prefix"},
	"str.ends_with":   {Sig: "str.ends_with(s, sfx)", Detail: "alias of has_suffix"},
	"str.slice":       {Sig: "str.slice(s, start, end?)", Detail: "substring"},
	"str.lines":       {Sig: "str.lines(s)", Detail: "split lines"},
	"re.find":         {Sig: "re.find(pattern, s)", Detail: "first match"},
	"re.find_all":     {Sig: "re.find_all(pattern, s)", Detail: "all matches"},
	"re.is_match":     {Sig: "re.is_match(pattern, s) -> bool", Detail: "match?"},
	"re.replace":      {Sig: "re.replace(pattern, s, repl)", Detail: "regex replace"},
	"re.split":        {Sig: "re.split(pattern, s)", Detail: "regex split"},
	"math.abs":        {Sig: "math.abs(n)", Detail: "absolute value"},
	"math.min":        {Sig: "math.min(a, b, …)", Detail: "minimum"},
	"math.max":        {Sig: "math.max(a, b, …)", Detail: "maximum"},
	"math.clamp":      {Sig: "math.clamp(n, lo, hi)", Detail: "clamp range"},
	"math.pow":        {Sig: "math.pow(base, exp)", Detail: "power"},
	"math.sqrt":       {Sig: "math.sqrt(n)", Detail: "square root"},
	"math.floor":      {Sig: "math.floor(n)", Detail: "floor"},
	"math.ceil":       {Sig: "math.ceil(n)", Detail: "ceiling"},
	"math.sum":        {Sig: "math.sum(list)", Detail: "sum numbers"},
	"math.mean":       {Sig: "math.mean(list)", Detail: "average"},
	"math.median":     {Sig: "math.median(list)", Detail: "median"},
	"math.pi":         {Sig: "math.pi", Detail: "π constant"},
	"math.e":          {Sig: "math.e", Detail: "e constant"},

	// test
	"test.eq":       {Sig: "test.eq(a, b)", Detail: "assert equal"},
	"test.ne":       {Sig: "test.ne(a, b)", Detail: "assert not equal"},
	"test.is_true":  {Sig: "test.is_true(v)", Detail: "assert truthy"},
	"test.is_false": {Sig: "test.is_false(v)", Detail: "assert falsey"},
	"test.ok":       {Sig: "test.ok(result)", Detail: "assert Result ok"},
	"test.err":      {Sig: "test.err(result)", Detail: "assert Result err"},
	"test.contains": {Sig: "test.contains(hay, needle)", Detail: "string/list contains"},
	"test.approx":   {Sig: "test.approx(a, b, eps?)", Detail: "float near-equal"},
	"test.is_null":  {Sig: "test.is_null(v)", Detail: "assert null"},
	"test.fail":     {Sig: "test.fail(msg)", Detail: "hard fail"},
	"test.skip":     {Sig: "test.skip(msg)", Detail: "skip test"},

	// local LLM
	"ollama.chat":     {Sig: "ollama.chat(prompt | opts) -> Result", Detail: "local Ollama chat"},
	"ollama.list":     {Sig: "ollama.list() -> Result", Detail: "list models"},
	"ollama.generate": {Sig: "ollama.generate(prompt | opts) -> Result", Detail: "generate"},
	"ollama.ps":       {Sig: "ollama.ps() -> Result", Detail: "running models"},
	"ollama.pull":     {Sig: "ollama.pull(model) -> Result", Detail: "pull model"},
	"ollama.host":     {Sig: "ollama.host()", Detail: "Ollama base URL"},
	"vllm.chat":       {Sig: "vllm.chat(opts) -> Result", Detail: "vLLM chat/completions"},
	"vllm.list":       {Sig: "vllm.list() -> Result", Detail: "list models"},
	"vllm.health":     {Sig: "vllm.health() -> Result", Detail: "health check"},
	"vllm.connect":    {Sig: "vllm.connect(url?)", Detail: "set base URL"},

	// misc glue
	"url.parse":           {Sig: "url.parse(s) -> Result", Detail: "parse URL"},
	"url.build":           {Sig: "url.build(parts)", Detail: "build URL"},
	"url.join":            {Sig: "url.join(base, ref)", Detail: "resolve URL"},
	"uuid.v4":             {Sig: "uuid.v4()", Detail: "random UUID"},
	"base64.encode":       {Sig: "base64.encode(s)", Detail: "base64 encode"},
	"base64.decode":       {Sig: "base64.decode(s) -> Result", Detail: "base64 decode"},
	"crypto.sha256":       {Sig: "crypto.sha256(s)", Detail: "SHA-256 hex"},
	"crypto.argon2id":     {Sig: "crypto.argon2id(password, salt, time?, memory?, threads?, keyLen?)", Detail: "Argon2id password hash → hex"},
	"crypto.pbkdf2":       {Sig: "crypto.pbkdf2(password, salt, iterations?, keyLen?, algo?)", Detail: "PBKDF2-HMAC key derivation → hex"},
	"crypto.uuid":         {Sig: "crypto.uuid()", Detail: "UUID helper"},
	"archive.zip":         {Sig: "archive.zip(src, dest) -> Result", Detail: "zip path"},
	"archive.unzip":       {Sig: "archive.unzip(src, dest) -> Result", Detail: "unzip"},
	"platform.os":         {Sig: "platform.os()", Detail: "GOOS"},
	"platform.arch":       {Sig: "platform.arch()", Detail: "GOARCH"},
	"platform.cpus":       {Sig: "platform.cpus()", Detail: "CPU count"},
	"iter.chain":          {Sig: "iter.chain(lists…)", Detail: "chain sequences"},
	"iter.take":           {Sig: "iter.take(list, n)", Detail: "first n"},
	"iter.drop":           {Sig: "iter.drop(list, n)", Detail: "drop first n"},
	"iter.chunk":          {Sig: "iter.chunk(list, n)", Detail: "chunk list"},
	"collections.counter": {Sig: "collections.counter(list)", Detail: "value counts"},
	"heap.nsmallest":      {Sig: "heap.nsmallest(n, list)", Detail: "n smallest"},
	"heap.nlargest":       {Sig: "heap.nlargest(n, list)", Detail: "n largest"},
	"bisect.insort":       {Sig: "bisect.insort(list, x)", Detail: "insert sorted"},
	"pipe.reduce":         {Sig: "pipe.reduce(list, fn, init?)", Detail: "reduce"},
	"viz.bar":             {Sig: "viz.bar(data, opts?)", Detail: "bar chart SVG"},
	"viz.line":            {Sig: "viz.line(data, opts?)", Detail: "line chart SVG"},
	"viz.save":            {Sig: "viz.save(path, svg) -> Result", Detail: "write chart"},
	"graphql.query":       {Sig: "graphql.query(url, query, vars?) -> Result", Detail: "GraphQL query"},
}

func lookupMember(pkg, mem string) (memberHelp, bool) {
	if mem == "" {
		h, ok := memberCatalog[pkg]
		return h, ok
	}
	h, ok := memberCatalog[pkg+"."+mem]
	return h, ok
}

func lookupCall(name string) (memberHelp, bool) {
	if h, ok := memberCatalog[name]; ok {
		return h, true
	}
	return memberHelp{}, false
}

// LookupMemberHelp returns the sig and detail for a "pkg.member" key.
func LookupMemberHelp(key string) (string, string) {
	if h, ok := memberCatalog[key]; ok {
		return h.Sig, h.Detail
	}
	return "", ""
}

func init() {
	// sysinfo
	memberCatalog["sysinfo.memory"] = memberHelp{"sysinfo.memory() -> Result[map]", "memory byte counts plus human-readable sizes; percent is used memory"}
	memberCatalog["sysinfo.disk"] = memberHelp{"sysinfo.disk(path?) -> Result[map]", "filesystem byte counts; free is available to the current user"}
	memberCatalog["sysinfo.uptime"] = memberHelp{"sysinfo.uptime() -> Result[map]", "host uptime where supported: seconds and rounded human duration"}
	memberCatalog["sysinfo.loadavg"] = memberHelp{"sysinfo.loadavg() -> Result[[float]]", "load averages in order: 1 minute, 5 minutes, 15 minutes"}
	memberCatalog["sysinfo.cpu_count"] = memberHelp{"sysinfo.cpu_count() -> int", "logical CPUs visible to the current process"}
	memberCatalog["sysinfo.net_interfaces"] = memberHelp{"sysinfo.net_interfaces() -> Result[[map]]", "network interfaces: name, mac, mtu, addrs, up"}
	memberCatalog["sysinfo.env_summary"] = memberHelp{"sysinfo.env_summary() -> map", "os, arch, cpus, hostname, user, home, pid"}

	// proc
	memberCatalog["proc.self"] = memberHelp{"proc.self() -> map", "current process IDs plus user/group names when available"}
	memberCatalog["proc.list"] = memberHelp{"proc.list() -> Result[[map]]", "processes: pid, executable name, command where visible"}
	memberCatalog["proc.find"] = memberHelp{"proc.find(name) -> Result[[map]]", "case-insensitive substring search in process name or command"}
	memberCatalog["proc.kill"] = memberHelp{"proc.kill(pid, signal?) -> Result", "send a signal to a positive process ID (default SIGTERM)"}
	memberCatalog["proc.exists"] = memberHelp{"proc.exists(pid) -> bool", "best-effort signal-0 check for a positive process ID"}

	// netutil
	memberCatalog["netutil.port_open"] = memberHelp{"netutil.port_open(host, port, timeout?) -> Result[bool]", "check if TCP port is open"}
	memberCatalog["netutil.tcp_ping"] = memberHelp{"netutil.tcp_ping(host, port, timeout?) -> Result[map]", "TCP ping: open, latency_ms"}
	memberCatalog["netutil.resolve"] = memberHelp{"netutil.resolve(host) -> Result[[str]]", "DNS lookup → IP addresses"}
	memberCatalog["netutil.lookup_host"] = memberHelp{"netutil.lookup_host(host) -> Result[[str]]", "DNS name lookup"}
	memberCatalog["netutil.lookup_txt"] = memberHelp{"netutil.lookup_txt(host) -> Result[[str]]", "DNS TXT records"}
	memberCatalog["netutil.lookup_mx"] = memberHelp{"netutil.lookup_mx(host) -> Result[[map]]", "DNS MX records"}
	memberCatalog["netutil.reverse_lookup"] = memberHelp{"netutil.reverse_lookup(ip) -> Result[[str]]", "reverse DNS"}
	memberCatalog["netutil.scan_ports"] = memberHelp{"netutil.scan_ports(host, [ports]) -> Result[[map]]", "scan multiple ports"}

	// mcp
	memberCatalog["mcp.connect"] = memberHelp{"mcp.connect(command, args?) -> Result[client]", "connect to MCP server (stdio)"}
	memberCatalog["mcp.connect_sse"] = memberHelp{"mcp.connect_sse(url) -> Result[client]", "connect to MCP server (HTTP+SSE)"}
	memberCatalog["mcp.tool"] = memberHelp{"mcp.tool(name, desc, fn, schema?) -> map", "define an MCP tool"}
	memberCatalog["mcp.serve_stdio"] = memberHelp{"mcp.serve_stdio([tools])", "run MCP server on stdio"}

	// deepgram
	memberCatalog["deepgram.stream"] = memberHelp{"deepgram.stream(opts?) -> Result[stream]", "streaming STT via WebSocket (Nova-2)"}
	memberCatalog["deepgram.transcribe"] = memberHelp{"deepgram.transcribe(url_or_file, opts?) -> Result[map]", "transcribe audio (REST)"}

	// elevenlabs
	memberCatalog["elevenlabs.stream"] = memberHelp{"elevenlabs.stream(text, opts?) -> Result[stream]", "streaming TTS via WebSocket"}
	memberCatalog["elevenlabs.stream_ws"] = memberHelp{"elevenlabs.stream_ws(opts?) -> Result[ws]", "bidirectional TTS (lowest latency)"}
	memberCatalog["elevenlabs.speak"] = memberHelp{"elevenlabs.speak(text, opts?) -> Result[map]", "synthesize audio (REST)"}
	memberCatalog["elevenlabs.voices"] = memberHelp{"elevenlabs.voices() -> Result[[map]]", "list available voices"}

	// mlinfer
	memberCatalog["mlinfer.predict"] = memberHelp{"mlinfer.predict(url, input, opts?) -> Result[any]", "POST JSON to an HTTP(S) endpoint; opts supports timeout, headers, api_key"}
	memberCatalog["mlinfer.onnx"] = memberHelp{"mlinfer.onnx(base, model, input) -> Result[any]", "ONNX Runtime Server v1 inference; input must be a map"}
	memberCatalog["mlinfer.triton"] = memberHelp{"mlinfer.triton(base, model, input) -> Result[any]", "NVIDIA Triton v2 inference; input must be a map"}
	memberCatalog["mlinfer.hf"] = memberHelp{"mlinfer.hf(model, input, opts?) -> Result[any]", "HuggingFace Inference API; opts may provide api_key"}
	memberCatalog["mlinfer.classify"] = memberHelp{"mlinfer.classify(url, text, opts?) -> Result[any]", "POST text to a classification endpoint"}
	memberCatalog["mlinfer.embed"] = memberHelp{"mlinfer.embed(url, text, opts?) -> Result[any]", "POST text to an embedding endpoint"}
	memberCatalog["mlinfer.detect"] = memberHelp{"mlinfer.detect(url, image_url) -> Result[any]", "POST an image URL to an object-detection endpoint"}
	memberCatalog["mlinfer.batch"] = memberHelp{"mlinfer.batch(url, [inputs]) -> Result[any]", "POST a JSON array of inputs"}
	memberCatalog["mlinfer.onnx_health"] = memberHelp{"mlinfer.onnx_health(base) -> Result[bool]", "ONNX health: true only for HTTP 200"}
	memberCatalog["mlinfer.triton_health"] = memberHelp{"mlinfer.triton_health(base) -> Result[bool]", "Triton readiness: true only for HTTP 200"}

	// governor
	memberCatalog["governor.new"] = memberHelp{"governor.new(opts) -> map", "create execution governor (token/time/cost budgets)"}

	// supervisor
	memberCatalog["supervisor.new"] = memberHelp{"supervisor.new(opts) -> map", "create supervisor (one_for_one/all/rest_for_one)"}
	memberCatalog["supervisor.process"] = memberHelp{"supervisor.process(fn, opts?) -> map", "create isolated process with mailbox"}

	// cluster
	memberCatalog["cluster.store"] = memberHelp{"cluster.store(redis_url) -> Result[map]", "connect to shared state store (Redis)"}
	memberCatalog["cluster.lock"] = memberHelp{"cluster.lock(store, key, opts?) -> Result[lock]", "distributed lock with TTL"}
	memberCatalog["cluster.register"] = memberHelp{"cluster.register(store, node_id, opts?) -> Result", "register node with heartbeat"}
	memberCatalog["cluster.nodes"] = memberHelp{"cluster.nodes(store) -> Result[[map]]", "list active nodes"}
	memberCatalog["cluster.rate_limit"] = memberHelp{"cluster.rate_limit(store, key, max, window) -> Result[bool]", "distributed rate limiter"}
	memberCatalog["cluster.counter"] = memberHelp{"cluster.counter(store, key) -> Result[counter]", "distributed atomic counter"}
	memberCatalog["cluster.publish"] = memberHelp{"cluster.publish(store, channel, msg) -> Result", "pub/sub publish"}

	// ratelimit
	memberCatalog["ratelimit.new"] = memberHelp{"ratelimit.new(rate, unit) -> Result[limiter]", "token bucket rate limiter"}
	memberCatalog["ratelimit.wait"] = memberHelp{"ratelimit.wait(rl)", "block until token available"}
	memberCatalog["ratelimit.acquire"] = memberHelp{"ratelimit.acquire(rl) -> bool", "non-blocking acquire"}

	// migrate
	memberCatalog["migrate.run"] = memberHelp{"migrate.run(conn, dir) -> Result[int]", "apply pending migrations"}
	memberCatalog["migrate.status"] = memberHelp{"migrate.status(conn, dir) -> Result[list]", "migration status"}
	memberCatalog["migrate.create"] = memberHelp{"migrate.create(dir, name) -> Result[str]", "create timestamped .sql file"}

	// metrics
	memberCatalog["metrics.accuracy"] = memberHelp{"metrics.accuracy(y_true, y_pred) -> counter", "accuracy score"}
	memberCatalog["metrics.f1"] = memberHelp{"metrics.f1(y_true, y_pred) -> gauge", "F1 score"}
	memberCatalog["metrics.precision"] = memberHelp{"metrics.precision(y_true, y_pred) -> histogram", "precision score"}

	// email
	memberCatalog["email.send"] = memberHelp{"email.send(to, subject, body, opts?) -> Result", "send email via SMTP"}
	memberCatalog["email.parse"] = memberHelp{"email.parse(raw) -> Result[map]", "parse email message"}

	// socket
	memberCatalog["socket.dial"] = memberHelp{"socket.dial(network, addr, timeout?) -> Result[conn]", "TCP/UDP connection (SSRF-guarded)"}
	memberCatalog["socket.listen"] = memberHelp{"socket.listen(network, addr) -> Result[listener]", "bind TCP/UDP listener"}
	memberCatalog["socket.resolve"] = memberHelp{"socket.resolve(host) -> Result[[str]]", "DNS lookup → IPs"}

	// pcap
	memberCatalog["pcap.ethernet"] = memberHelp{"pcap.ethernet(opts) -> bytes", "build Ethernet frame"}
	memberCatalog["pcap.ipv4"] = memberHelp{"pcap.ipv4(opts) -> bytes", "build IPv4 packet"}
	memberCatalog["pcap.tcp"] = memberHelp{"pcap.tcp(opts) -> bytes", "build TCP segment"}
	memberCatalog["pcap.udp"] = memberHelp{"pcap.udp(opts) -> bytes", "build UDP datagram"}
	memberCatalog["pcap.write"] = memberHelp{"pcap.write(path, [pkts]) -> Result", "write PCAP file"}
	memberCatalog["pcap.read"] = memberHelp{"pcap.read(path) -> Result[[map]]", "read PCAP file"}

	// shlex
	memberCatalog["shlex.split"] = memberHelp{"shlex.split(s) -> [str]", "POSIX shell-style split"}
	memberCatalog["shlex.quote"] = memberHelp{"shlex.quote(s) -> str", "shell-safe quote"}
	memberCatalog["shlex.join"] = memberHelp{"shlex.join([args]) -> str", "join into shell command"}

	// signal
	memberCatalog["signal.listen"] = memberHelp{"signal.listen()", "start watching SIGINT/SIGTERM"}
	memberCatalog["signal.received"] = memberHelp{"signal.received(name?) -> bool", "check if signal received"}
	memberCatalog["signal.reset"] = memberHelp{"signal.reset()", "clear signal flags"}

	// binstruct
	memberCatalog["binstruct.pack"] = memberHelp{"binstruct.pack(fmt, values) -> Result[str]", "pack binary data"}
	memberCatalog["binstruct.unpack"] = memberHelp{"binstruct.unpack(fmt, data) -> Result[[any]]", "unpack binary data"}
	memberCatalog["binstruct.size"] = memberHelp{"binstruct.size(fmt) -> int", "packed size in bytes"}

	// difflib
	memberCatalog["difflib.unified_diff"] = memberHelp{"difflib.unified_diff(a, b, opts?) -> str", "unified diff"}
	memberCatalog["difflib.ndiff"] = memberHelp{"difflib.ndiff(a, b) -> str", "ndiff comparison"}

	// copy
	memberCatalog["copy.copy"] = memberHelp{"copy.copy(v) -> any", "shallow copy"}
	memberCatalog["copy.deepcopy"] = memberHelp{"copy.deepcopy(v) -> any", "deep copy"}

	// functools
	memberCatalog["functools.partial"] = memberHelp{"functools.partial(fn, args…) -> fn", "partial application"}
	memberCatalog["functools.once"] = memberHelp{"functools.once(fn) -> fn", "call only once, cache result"}

	// traceback
	memberCatalog["traceback.format"] = memberHelp{"traceback.format(err) -> str", "format error with traceback"}

	// pickle
	memberCatalog["pickle.dumps"] = memberHelp{"pickle.dumps(v) -> str", "serialize to pickle-like format"}
	memberCatalog["pickle.loads"] = memberHelp{"pickle.loads(s) -> any", "deserialize"}

	// decimal
	memberCatalog["decimal.new"] = memberHelp{"decimal.new(v) -> decimal", "create decimal number"}
	memberCatalog["decimal.add"] = memberHelp{"decimal.add(a, b) -> decimal", "add decimals"}
	memberCatalog["decimal.sub"] = memberHelp{"decimal.sub(a, b) -> decimal", "subtract"}
	memberCatalog["decimal.mul"] = memberHelp{"decimal.mul(a, b) -> decimal", "multiply"}
	memberCatalog["decimal.div"] = memberHelp{"decimal.div(a, b) -> Result[decimal]", "divide"}
	memberCatalog["decimal.abs"] = memberHelp{"decimal.abs(d) -> decimal", "absolute value"}

	// random
	memberCatalog["random.int"] = memberHelp{"random.int(min, max) -> int", "random integer in range"}
	memberCatalog["random.float"] = memberHelp{"random.float() -> float", "random float 0.0-1.0"}
	memberCatalog["random.choice"] = memberHelp{"random.choice(list) -> any", "random element"}
	memberCatalog["random.shuffle"] = memberHelp{"random.shuffle(list) -> [any]", "shuffle list"}
	memberCatalog["random.bytes"] = memberHelp{"random.bytes(n) -> str", "random bytes"}

	// ip
	memberCatalog["ip.parse"] = memberHelp{"ip.parse(s) -> Result[map]", "parse IP address"}
	memberCatalog["ip.network"] = memberHelp{"ip.network(cidr) -> Result[map]", "parse CIDR network"}
	memberCatalog["ip.in_network"] = memberHelp{"ip.in_network(ip, cidr) -> bool", "check if IP is in network"}

	// mime
	memberCatalog["mime.by_ext"] = memberHelp{"mime.by_ext(ext) -> str", "MIME type by file extension"}
	memberCatalog["mime.ext"] = memberHelp{"mime.ext(mime) -> str", "file extension for MIME type"}

	// html
	memberCatalog["html.escape"] = memberHelp{"html.escape(s) -> str", "HTML-escape special chars"}
	memberCatalog["html.strip_tags"] = memberHelp{"html.strip_tags(s) -> str", "remove HTML tags"}
	memberCatalog["html.links"] = memberHelp{"html.links(html) -> [str]", "extract URLs from HTML"}

	// base64
	memberCatalog["base64.encode"] = memberHelp{"base64.encode(s) -> str", "base64 encode"}
	memberCatalog["base64.decode"] = memberHelp{"base64.decode(s) -> Result[str]", "base64 decode"}
	memberCatalog["base64.url_encode"] = memberHelp{"base64.url_encode(s) -> str", "URL-safe base64 encode"}
	memberCatalog["base64.url_decode"] = memberHelp{"base64.url_decode(s) -> Result[str]", "URL-safe base64 decode"}

	// tokenizer
	memberCatalog["tokenizer.encode"] = memberHelp{"tokenizer.encode(text) -> [int]", "tokenize text"}
	memberCatalog["tokenizer.decode"] = memberHelp{"tokenizer.decode([ints]) -> str", "decode tokens"}
	memberCatalog["tokenizer.count"] = memberHelp{"tokenizer.count(text) -> int", "count tokens"}

	// dataset
	memberCatalog["dataset.stream"] = memberHelp{"dataset.stream(path) -> Result[list]", "stream JSONL rows"}
	memberCatalog["dataset.head"] = memberHelp{"dataset.head(data, n) -> Result", "first N rows"}
	memberCatalog["dataset.sample"] = memberHelp{"dataset.sample(data, n) -> [train, test]", "random sample of N rows"}

	// webrtc
	memberCatalog["webrtc.hub"] = memberHelp{"webrtc.hub() -> hub", "create signaling hub"}

	// encoding
	memberCatalog["encoding.hex_encode"] = memberHelp{"encoding.hex_encode(data) -> str", "hex encode"}
	memberCatalog["encoding.hex_decode"] = memberHelp{"encoding.hex_decode(s) -> Result[str]", "hex decode"}
	memberCatalog["encoding.base32_encode"] = memberHelp{"encoding.base32_encode(data) -> str", "base32 encode"}
	memberCatalog["encoding.base32_decode"] = memberHelp{"encoding.base32_decode(s) -> Result[str]", "base32 decode"}
	memberCatalog["encoding.url_encode"] = memberHelp{"encoding.url_encode(s) -> str", "URL query encode"}
	memberCatalog["encoding.url_decode"] = memberHelp{"encoding.url_decode(s) -> Result[str]", "URL query decode"}
	memberCatalog["encoding.path_encode"] = memberHelp{"encoding.path_encode(s) -> str", "URL path encode"}
	memberCatalog["encoding.path_decode"] = memberHelp{"encoding.path_decode(s) -> Result[str]", "URL path decode"}
	memberCatalog["encoding.to_hex"] = memberHelp{"encoding.to_hex(n) -> str", "number/bytes to hex"}

	// compress
	memberCatalog["compress.gzip"] = memberHelp{"compress.gzip(data) -> Result[str]", "gzip compress"}
	memberCatalog["compress.gunzip"] = memberHelp{"compress.gunzip(data) -> Result[str]", "gzip decompress"}
	memberCatalog["compress.deflate"] = memberHelp{"compress.deflate(data) -> Result[str]", "zlib compress"}
	memberCatalog["compress.inflate"] = memberHelp{"compress.inflate(data) -> Result[str]", "zlib decompress"}

	// dns
	memberCatalog["dns.lookup"] = memberHelp{"dns.lookup(host) -> Result[[str]]", "DNS A/AAAA lookup"}
	memberCatalog["dns.srv"] = memberHelp{"dns.srv(service, proto, name) -> Result[[map]]", "SRV record lookup"}
	memberCatalog["dns.cname"] = memberHelp{"dns.cname(host) -> Result[str]", "CNAME lookup"}
	memberCatalog["dns.ns"] = memberHelp{"dns.ns(domain) -> Result[[str]]", "nameserver lookup"}
	memberCatalog["dns.mx"] = memberHelp{"dns.mx(domain) -> Result[[map]]", "MX record lookup"}
	memberCatalog["dns.txt"] = memberHelp{"dns.txt(domain) -> Result[[str]]", "TXT record lookup"}
	memberCatalog["dns.reverse"] = memberHelp{"dns.reverse(ip) -> Result[[str]]", "reverse DNS (PTR)"}

	// tls
	memberCatalog["tls.cert_info"] = memberHelp{"tls.cert_info(host) -> Result[map]", "TLS certificate details"}
	memberCatalog["tls.verify"] = memberHelp{"tls.verify(host) -> Result[map]", "verify TLS certificate validity"}
	memberCatalog["tls.chain"] = memberHelp{"tls.chain(host) -> Result[[map]]", "certificate chain"}
	memberCatalog["tls.expiry_check"] = memberHelp{"tls.expiry_check(host, warn_days?) -> Result[map]", "check cert expiry with warning threshold"}
	memberCatalog["tls.supported_versions"] = memberHelp{"tls.supported_versions() -> [str]", "list supported TLS versions"}
	memberCatalog["tls.system_roots"] = memberHelp{"tls.system_roots() -> Result[int]", "count system root CAs"}

	// os
	memberCatalog["os.getenv"] = memberHelp{"os.getenv(key) -> str?", "get environment variable"}
	memberCatalog["os.setenv"] = memberHelp{"os.setenv(key, val) -> Result", "set environment variable"}
	memberCatalog["os.unsetenv"] = memberHelp{"os.unsetenv(key) -> Result", "remove environment variable"}
	memberCatalog["os.environ"] = memberHelp{"os.environ() -> map", "all environment variables"}
	memberCatalog["os.cwd"] = memberHelp{"os.cwd() -> str", "current working directory"}
	memberCatalog["os.chdir"] = memberHelp{"os.chdir(path) -> Result", "change working directory"}
	memberCatalog["os.hostname"] = memberHelp{"os.hostname() -> str", "system hostname"}
	memberCatalog["os.pid"] = memberHelp{"os.pid() -> int", "current process ID"}
	memberCatalog["os.user"] = memberHelp{"os.user() -> Result[map]", "current user info"}
	memberCatalog["os.home"] = memberHelp{"os.home() -> str", "user home directory"}
	memberCatalog["os.temp_dir"] = memberHelp{"os.temp_dir() -> str", "system temp directory"}
	memberCatalog["os.args"] = memberHelp{"os.args() -> [str]", "command-line arguments"}
	memberCatalog["os.platform"] = memberHelp{"os.platform() -> map", "OS/arch/CPU info"}
	memberCatalog["os.path_join"] = memberHelp{"os.path_join(parts...) -> str", "join path components"}
	memberCatalog["os.path_exists"] = memberHelp{"os.path_exists(path) -> bool", "check if path exists"}
	memberCatalog["os.mkdir"] = memberHelp{"os.mkdir(path) -> Result", "create directory (recursive)"}
	memberCatalog["os.remove"] = memberHelp{"os.remove(path) -> Result", "remove single file or empty directory"}
	memberCatalog["os.remove_tree"] = memberHelp{"os.remove_tree(path) -> Result", "recursively remove directory tree (refuses / and .)"}
	memberCatalog["os.rename"] = memberHelp{"os.rename(old, new) -> Result", "rename file or directory"}
	memberCatalog["os.stat"] = memberHelp{"os.stat(path) -> Result[map]", "file info (size, mode, time)"}
	memberCatalog["os.chmod"] = memberHelp{"os.chmod(path, mode) -> Result", "change file permissions"}
	memberCatalog["os.separator"] = memberHelp{"os.separator() -> str", "path separator for this OS"}
}
