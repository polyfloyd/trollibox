const path = require('path');
const webpack = require('webpack');
const CopyPlugin = require("copy-webpack-plugin");
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const { VueLoaderPlugin } = require('vue-loader');


let pages = [
  {chunk: 'index', file: './view/index'},
];


module.exports = {
  resolve: {
    alias: {
      // TODO: Use vue-loader.
      'vue$': 'vue/dist/vue.esm-bundler.js',
    },
  },
  entry: pages
    .map((page) => {
      let e = {};
      e[page.chunk] = [`${page.file}.js`];
      return e;
    })
    .reduce((e, o) => { return {...e, ...o}; }, {}),
  output: {
    filename: '[name].bundle.js',
    publicPath: '../static/',
    clean: true,
  },
  plugins: [
    ...pages.map((page) => new HtmlWebpackPlugin({
      template: `${page.file}.html`,
      filename: `${page.chunk}.html`,
      chunks: [ page.chunk ],
      inject: false,
      scriptLoading: 'blocking',
      minify: false,
    })),
    new MiniCssExtractPlugin({
      filename: '[name].css',
      chunkFilename: '[id].css',
    }),
    new CopyPlugin({
      patterns: [
        { from: './static/default-album-art.svg', to: 'default-album-art.svg' },
        { from: './static/favicon.ico', to: 'favicon.ico' },
        { from: './static/robots.txt', to: 'robots.txt' },
      ],
    }),
    new VueLoaderPlugin(),
    new webpack.DefinePlugin({
      __VUE_OPTIONS_API__: true,
      __VUE_PROD_DEVTOOLS__: false,
    }),
  ],
  module: {
    rules: [
      {
        test: /\.vue$/,
        loader: 'vue-loader',
      },
      {
        test: /\.js$/,
        exclude: path.resolve(__dirname, './node_modules'),
        loader: 'babel-loader',
        options: {
          presets: [ '@babel/preset-env' ],
        },
      },
      {
        test: /\.s?css$/,
        use: [
          MiniCssExtractPlugin.loader,
          'css-loader',
          'sass-loader',
        ],
      },
      {
        test: /\.(svg|png|jpg|gif|woff|woff2|eot|ttf|otf)$/,
        type: 'asset/resource',
        generator: {
          filename: 'static/[name]_[hash][ext][query]',
        }
      },
    ]
  }
};
