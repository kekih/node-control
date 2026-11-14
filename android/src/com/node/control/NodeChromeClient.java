package com.node.control;

import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebView;

public class NodeChromeClient extends WebChromeClient {
    private final MainActivity activity;

    public NodeChromeClient(MainActivity activity) {
        this.activity = activity;
    }

    @Override
    @SuppressWarnings({"rawtypes", "unchecked"})
    public boolean onShowFileChooser(WebView view, ValueCallback filePathCallback, FileChooserParams params) {
        return activity.handleFileChooser(filePathCallback, params);
    }
}
