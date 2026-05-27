-- Dev gate: detect duplicate lazy.nvim `keys` declarations.
--
-- Two plugin specs declaring the same lhs+mode is silent in Vim — the spec
-- that loads last wins and the other never fires. This enumerates every
-- declared key across all lazy specs (by lhs+mode, mirroring lazy's own
-- handler) and fails if any pair collides.
--
-- Run against the applied tree:
--   nvim --headless -c 'luafile ~/.config/nvim/scripts/keymap-check.lua' -c 'qa!'
-- Exits non-zero (via :cq) on conflict.

local Keys = require("lazy.core.handler.keys")
local plugins = require("lazy.core.config").plugins

local seen = {}
for name, pl in pairs(plugins) do
	for _, value in ipairs(pl.keys or {}) do
		if type(value) == "string" then
			value = { value }
		end
		local mode_spec = value.mode or "n"
		local modes = type(mode_spec) == "table" and mode_spec or { mode_spec }
		for _, mode in ipairs(modes) do
			local ok, parsed = pcall(Keys.parse, value, mode)
			if ok and parsed.lhs then
				seen[parsed.id] = seen[parsed.id] or {}
				table.insert(seen[parsed.id], name)
			end
		end
	end
end

local conflicts = {}
for id, srcs in pairs(seen) do
	if #srcs > 1 then
		table.insert(conflicts, ("  %q claimed by: %s"):format(id, table.concat(srcs, ", ")))
	end
end

if #conflicts > 0 then
	table.sort(conflicts)
	io.stderr:write("Keymap conflicts found:\n" .. table.concat(conflicts, "\n") .. "\n")
	vim.cmd("cq")
else
	print("No keymap conflicts.")
end
