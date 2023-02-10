use std::fs;

fn main() {
    println!("Content-Type: text/plain");
    println!("");
    let paths = fs::read_dir("./").unwrap();
    for path in paths{
        println!("{}", path.unwrap().path().display())
    }
}
