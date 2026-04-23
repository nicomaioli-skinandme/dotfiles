return {
  {
    "catppuccin/nvim",
    name = "catppuccin",
    lazy = false,
    priority = 1000,
    opts = {
      flavour = "mocha",
      background = { light = "latte", dark = "mocha" },
      integrations = {
        treesitter = true,
        telescope = { enabled = true },
        neotree = true,
        which_key = true,
        mason = true,
        native_lsp = { enabled = true },
        blink_cmp = true,
      },
    },
    config = function(_, opts)
      require("catppuccin").setup(opts)
      vim.cmd.colorscheme("catppuccin-mocha")
    end,
  },
}
