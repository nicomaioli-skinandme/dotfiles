return {
	{
		"aserowy/tmux.nvim",
		event = "VeryLazy",
		opts = {
			copy_sync = {
				enable = true,
			},
			navigation = {
				enable_default_keybindings = true,
				cycle_navigation = true,
			},
			resize = {
				enable_default_keybindings = false,
			},
		},
		config = function(_, opts)
			require("tmux").setup(opts)
		end,
	},
}
