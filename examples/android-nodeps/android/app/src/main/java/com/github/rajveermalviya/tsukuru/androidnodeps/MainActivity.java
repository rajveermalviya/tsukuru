package com.github.rajveermalviya.tsukuru.androidnodeps;

import android.os.Bundle;
import android.app.Activity;
import android.widget.TextView;

public class MainActivity extends Activity {
	@Override
	protected void onCreate(Bundle savedInstanceState) {
		super.onCreate(savedInstanceState);
		setContentView(R.layout.activity_main);

		((TextView)findViewById(R.id.greeting)).setText(greeter());
	}

	private native String greeter();

	static {
		System.loadLibrary("main");
	}
}
