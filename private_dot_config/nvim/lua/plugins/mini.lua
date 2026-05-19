return {
	{
		"nvim-mini/mini.pairs",
		version = false,
		event = "InsertEnter",
		opts = {},
	},
	{
		"nvim-mini/mini.surround",
		version = false,
		event = "VeryLazy",
		opts = {},
	},
	{
		"nvim-mini/mini.statusline",
		version = false,
		event = "VeryLazy",
		opts = {
			use_icons = true,
		},
	},
	{
		"nvim-mini/mini.diff",
		version = false,
		event = { "BufReadPre", "BufNewFile" },
		opts = {
			view = {
				style = "sign",
			},
		},
		keys = {
			{
				"<leader>do",
				function()
					require("mini.diff").toggle_overlay(0)
				end,
				desc = "Toggle mini.diff overlay",
			},
		},
	},
	{
		"nvim-mini/mini.map",
		version = false,
		dependencies = { "nvim-mini/mini.diff" },
		keys = {
			{
				"<leader>mm",
				function()
					require("mini.map").toggle()
				end,
				desc = "Toggle minimap",
			},
		},
		config = function()
			local map = require("mini.map")
			map.setup({
				integrations = {
					map.gen_integration.builtin_search(),
					map.gen_integration.diff(),
					map.gen_integration.diagnostic(),
				},
			})
		end,
	},
}
