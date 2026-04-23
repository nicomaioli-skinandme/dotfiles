return {
  {
    "pwntester/octo.nvim",
    cmd = "Octo",
    dependencies = {
      "nvim-lua/plenary.nvim",
      "nvim-telescope/telescope.nvim",
      "nvim-tree/nvim-web-devicons",
    },
    keys = {
      { "<leader>oo", "<cmd>Octo<cr>",               desc = "Octo (GitHub) menu" },
      { "<leader>op", "<cmd>Octo pr list<cr>",       desc = "List PRs" },
      { "<leader>oP", "<cmd>Octo pr search<cr>",     desc = "Search PRs" },
      { "<leader>oi", "<cmd>Octo issue list<cr>",    desc = "List issues" },
      { "<leader>oI", "<cmd>Octo issue search<cr>",  desc = "Search issues" },
      { "<leader>or", "<cmd>Octo review start<cr>",  desc = "Start PR review" },
      { "<leader>oR", "<cmd>Octo review resume<cr>", desc = "Resume PR review" },
    },
    opts = {
      picker = "telescope",
      enable_builtin = true,
      default_merge_method = "squash",
    },
    config = function(_, opts)
      require("octo").setup(opts)
    end,
  },
}
