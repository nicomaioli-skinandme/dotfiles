return {
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
}
