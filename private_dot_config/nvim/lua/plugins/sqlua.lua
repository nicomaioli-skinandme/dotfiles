return {
	{
		"Xemptuous/sqlua.nvim",
		lazy = true,
		cmd = { "SQLua", "SQLuaEdit" },
		config = function()
			require("sqlua").setup({
				keybinds = {
					activate_db = "<leader>a",
				},
			})

			-- Append --ssl-verify-server-cert=0 to MySQL/MariaDB connections.
			-- The plugin doesn't expose a hook for extra CLI args, and the URL
			-- query-string path mangles values with a leading space.
			local Mysql = require("sqlua.connectors.mysql")
			local orig_setup = Mysql.setup
			function Mysql:setup(name, url, options)
				local s = orig_setup(self, name, url, options)
				table.insert(s.cli_args, "--ssl-verify-server-cert=0")
				return s
			end
		end,
	},
}
