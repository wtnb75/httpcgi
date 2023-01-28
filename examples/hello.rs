use std::env;

fn main() {
    println!("Content-Type: text/plain");
    println!("");
    for (key, value) in env::vars(){
        println!("{key}={value}");
    }
    eprintln!("finished(stderr).");
}
