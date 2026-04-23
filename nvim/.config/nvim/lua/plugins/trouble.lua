return {
  {
    "folke/trouble.nvim",
    cmd = { "Trouble" },
    opts = {},
    keys = {
      { "<leader>xx", "<cmd>Trouble diagnostics toggle<cr>",                       desc = "Diagnostics (Trouble)" },
      { "<leader>xX", "<cmd>Trouble diagnostics toggle filter.buf=0<cr>",          desc = "Buffer diagnostics (Trouble)" },
      { "<leader>xs", "<cmd>Trouble symbols toggle focus=false<cr>",               desc = "Symbols (Trouble)" },
      { "<leader>xl", "<cmd>Trouble lsp toggle focus=false win.position=right<cr>", desc = "LSP refs/defs/impls (Trouble)" },
      { "<leader>xL", "<cmd>Trouble loclist toggle<cr>",                           desc = "Location list (Trouble)" },
      { "<leader>xQ", "<cmd>Trouble qflist toggle<cr>",                            desc = "Quickfix (Trouble)" },
    },
  },
}
