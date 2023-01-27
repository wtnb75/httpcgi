fn main() {
    println!("Content-Type: text/plain");
    println!("");
    for i in 1..=3 {
        println!("count-{}", i);
    }

    print!("aaa");
    print!("bbb");
    println!("(EOF)");
}
